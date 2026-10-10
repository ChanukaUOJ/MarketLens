package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"marketlens-mcp/internal/auth"
	"marketlens-mcp/internal/backend"
	"marketlens-mcp/internal/tools"
)

func main() {
	var missing []string
	require := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			missing = append(missing, name)
		}
		return v
	}

	backendURL := require("BACKEND_API_URL")
	crawlerURL := require("CRAWLER_API_URL")
	issuer := require("THUNDER_ISSUER")
	resourceID := require("MCP_RESOURCE_ID")
	publicBaseURL := require("MCP_PUBLIC_BASE_URL")
	if len(missing) > 0 {
		log.Fatalf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// JWKS is fetched from THUNDER_BASE_URL when set, so in-cluster traffic can
	// use the service DNS name while tokens still carry the public issuer.
	jwksBaseURL := os.Getenv("THUNDER_BASE_URL")
	if jwksBaseURL == "" {
		jwksBaseURL = issuer
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	verifier := auth.NewVerifier(auth.Config{
		Issuer:        issuer,
		JWKSBaseURL:   jwksBaseURL,
		ResourceID:    resourceID,
		PublicBaseURL: publicBaseURL,
		InsecureTLS:   os.Getenv("THUNDER_INSECURE_TLS") == "true",
		Scopes:        []string{tools.SubmitVacanciesScope},
	})
	api := backend.NewClient(backendURL)
	server := tools.New(api, crawlerURL)

	// Stateless so any replica can serve any request - no sticky sessions needed.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.HandleFunc(auth.ProtectedResourcePath, verifier.ServeProtectedResourceMetadata)
	mux.Handle("/mcp", verifier.RequireValidToken(mcpHandler))

	// Kubernetes liveness/readiness probes
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := api.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"backend unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("MCP server listening on :%s/mcp (backend: %s)", port, backendURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("MCP server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down MCP server")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
