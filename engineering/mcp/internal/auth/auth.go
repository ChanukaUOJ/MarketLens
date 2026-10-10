package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const ProtectedResourcePath = "/.well-known/oauth-protected-resource/mcp"

// Config holds everything needed to validate MCP access tokens issued by ThunderID.
type Config struct {
	Issuer        string // THUNDER_ISSUER - expected "iss" claim
	JWKSBaseURL   string // THUNDER_BASE_URL - where /oauth2/jwks is fetched from
	ResourceID    string // MCP_RESOURCE_ID - expected "aud" claim
	PublicBaseURL string // MCP_PUBLIC_BASE_URL - advertised in WWW-Authenticate
	InsecureTLS   bool   // THUNDER_INSECURE_TLS
	Scopes        []string
}

type Verifier struct {
	cfg    Config
	client *http.Client

	jwksMu    sync.Mutex
	jwksCache []jwk
}

func NewVerifier(cfg Config) *Verifier {
	client := &http.Client{Timeout: 10 * time.Second}
	if cfg.InsecureTLS {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	return &Verifier{cfg: cfg, client: client}
}

type contextKey string

const scopesKey contextKey = "scopes"

// ScopesFromContext returns the scopes of the validated token for this request.
func ScopesFromContext(ctx context.Context) []string {
	scopes, _ := ctx.Value(scopesKey).([]string)
	return scopes
}

// ServeProtectedResourceMetadata serves the RFC 9728 metadata document MCP clients
// use to discover the authorization server.
func (v *Verifier) ServeProtectedResourceMetadata(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"resource":                 v.cfg.ResourceID,
		"authorization_servers":    []string{v.cfg.Issuer},
		"scopes_supported":         v.cfg.Scopes,
		"bearer_methods_supported": []string{"header"},
	})
}

// RequireValidToken rejects requests without a valid ThunderID bearer token and
// stores the token's scopes on the request context.
func (v *Verifier) RequireValidToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.Header().Set("WWW-Authenticate",
				`Bearer resource_metadata="`+v.cfg.PublicBaseURL+ProtectedResourcePath+`"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")

		_, scopes, err := v.verify(rawToken)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), scopesKey, scopes)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type jwtClaims struct {
	Iss   string          `json:"iss"`
	Sub   string          `json:"sub"`
	Exp   int64           `json:"exp"`
	Scope string          `json:"scope"`
	Aud   json.RawMessage `json:"aud"`
}

func (c *jwtClaims) audience() []string {
	if len(c.Aud) == 0 {
		return nil
	}
	var single string
	if json.Unmarshal(c.Aud, &single) == nil {
		return []string{single}
	}
	var many []string
	if json.Unmarshal(c.Aud, &many) == nil {
		return many
	}
	return nil
}

type parsedJWT struct {
	header       jwtHeader
	claims       jwtClaims
	signingInput string
	signature    []byte
}

func parseJWT(raw string) (*parsedJWT, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("bad token encoding")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("bad token encoding")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("bad token encoding")
	}
	var p parsedJWT
	if json.Unmarshal(headerBytes, &p.header) != nil || json.Unmarshal(payloadBytes, &p.claims) != nil {
		return nil, errors.New("bad token JSON")
	}
	p.signingInput = parts[0] + "." + parts[1]
	p.signature = sig
	return &p, nil
}

type jwk struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *Verifier) getJWKS() ([]jwk, error) {
	v.jwksMu.Lock()
	defer v.jwksMu.Unlock()
	if v.jwksCache != nil {
		return v.jwksCache, nil
	}
	resp, err := v.client.Get(v.cfg.JWKSBaseURL + "/oauth2/jwks")
	if err != nil {
		return nil, errors.New("cannot reach ThunderID JWKS")
	}
	defer resp.Body.Close()
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if json.NewDecoder(resp.Body).Decode(&doc) != nil {
		return nil, errors.New("bad JWKS document")
	}
	v.jwksCache = doc.Keys
	return v.jwksCache, nil
}

func publicKeyFromJWK(k jwk) (*rsa.PublicKey, error) {
	nBytes, err1 := base64.RawURLEncoding.DecodeString(k.N)
	eBytes, err2 := base64.RawURLEncoding.DecodeString(k.E)
	if err1 != nil || err2 != nil || len(eBytes) > 8 {
		return nil, errors.New("bad JWK")
	}
	padded := make([]byte, 8)
	copy(padded[8-len(eBytes):], eBytes)
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(binary.BigEndian.Uint64(padded))}, nil
}

// verify runs the full check: RS256 signature against ThunderID's JWKS,
// issuer match, expiry (30s clock skew), and audience match against this MCP
// server's own resource identifier (MCP_RESOURCE_ID) - NOT the REST API's
// THUNDER_AUDIENCE, since this is a different resource server.
func (v *Verifier) verify(raw string) (sub string, scopes []string, err error) {
	p, err := parseJWT(raw)
	if err != nil {
		return "", nil, err
	}
	if p.header.Alg != "RS256" {
		return "", nil, errors.New("unsupported token algorithm")
	}
	keys, err := v.getJWKS()
	if err != nil {
		return "", nil, err
	}
	var key *rsa.PublicKey
	for _, k := range keys {
		if k.Kid == p.header.Kid {
			if key, err = publicKeyFromJWK(k); err != nil {
				return "", nil, err
			}
			break
		}
	}
	if key == nil {
		return "", nil, errors.New("signing key not found")
	}
	digest := sha256.Sum256([]byte(p.signingInput))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], p.signature) != nil {
		return "", nil, errors.New("invalid token signature")
	}
	if p.claims.Iss != v.cfg.Issuer {
		return "", nil, errors.New("invalid token issuer")
	}
	if p.claims.Exp != 0 && p.claims.Exp < time.Now().Unix()-30 {
		return "", nil, errors.New("token has expired")
	}
	found := false
	for _, a := range p.claims.audience() {
		if a == v.cfg.ResourceID {
			found = true
		}
	}
	if !found {
		return "", nil, errors.New("invalid token audience")
	}
	return p.claims.Sub, strings.Fields(p.claims.Scope), nil
}
