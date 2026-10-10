package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client calls the MarketLens backend REST API. The MCP service never talks to
// the database directly - every tool goes through /api/v1.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient takes the backend's base URL without the /api/v1 prefix,
// e.g. "http://marketlens-backend:8080".
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Path joins path segments under /api/v1, escaping each one so tool input
// cannot change the route (e.g. a level of "../crawler").
func Path(segments ...string) string {
	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = url.PathEscape(s)
	}
	return "/" + strings.Join(escaped, "/")
}

// Get calls GET /api/v1{path}?{query} and returns the decoded JSON body.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (any, error) {
	u := c.baseURL + "/api/v1" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build backend request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach backend: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read backend response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var e struct {
			Error   string `json:"error"`
			Details string `json:"details"`
		}
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			if e.Details != "" {
				return nil, fmt.Errorf("%s: %s", e.Error, e.Details)
			}
			return nil, fmt.Errorf("%s", e.Error)
		}
		return nil, fmt.Errorf("backend returned status %d", resp.StatusCode)
	}

	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("invalid JSON from backend: %w", err)
	}
	return out, nil
}

// Ping checks that the backend process is up, for the MCP readiness probe.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend healthz returned %d", resp.StatusCode)
	}
	return nil
}
