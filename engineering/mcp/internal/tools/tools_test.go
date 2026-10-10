package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"marketlens-mcp/internal/backend"
)

// connect starts the MCP server against a fake backend and returns a client
// session plus a pointer to the last request URI the backend received.
func connect(t *testing.T, status int, body string) (*mcp.ClientSession, *string) {
	t.Helper()
	var lastURI string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastURI = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(fake.Close)

	ctx := context.Background()
	server := New(backend.NewClient(fake.URL), "http://crawler.invalid")
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session, &lastURI
}

func TestToolsCallBackendRoutes(t *testing.T) {
	session, lastURI := connect(t, http.StatusOK, `{"count":1}`)

	dates := map[string]any{"from_date": "2026-01-01", "to_date": "2026-03-31"}
	level := map[string]any{"standard": "occupation", "level": "major-group", "id": 2,
		"from_date": "2026-01-01", "to_date": "2026-03-31"}
	q := "?from-date=2026-01-01&to-date=2026-03-31"

	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"get_provinces", nil, "/api/v1/provinces"},
		{"get_job_types", nil, "/api/v1/job-types"},
		{"get_vocational_educations", nil, "/api/v1/vocational-educations"},
		{"get_major_group", map[string]any{"id": 7}, "/api/v1/major-groups/7"},
		{"get_occupation_groups", map[string]any{"limit": 5, "offset": 10}, "/api/v1/occupation-groups?limit=5&offset=10"},
		{"get_industry_subclasses", nil, "/api/v1/industry-subclasses"},
		{"get_hierarchy_children", level, "/api/v1/occupation/major-group/2/children" + q},
		{"get_remote_onsite_by_level", level, "/api/v1/occupation/major-group/2/remote-onsite-hybrid" + q},
		{"get_top_15_skills_by_occupation", map[string]any{"level": "unit-group", "id": 3,
			"from_date": "2026-01-01", "to_date": "2026-03-31"}, "/api/v1/occupation/unit-group/3/top-15-skills" + q},
		{"get_all_skills_by_occupation", map[string]any{"level": "unit-group", "id": 3, "limit": 50,
			"from_date": "2026-01-01", "to_date": "2026-03-31"}, "/api/v1/occupation/unit-group/3/all-skills?from-date=2026-01-01&limit=50&to-date=2026-03-31"},
		{"get_vacancy_trend", dates, "/api/v1/vacancy-trend" + q},
		{"get_industries_by_date_range", dates, "/api/v1/industries/by-date-range" + q},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			if res.IsError {
				t.Fatalf("tool returned error: %+v", res.Content)
			}
			if *lastURI != tc.want {
				t.Fatalf("backend got %q, want %q", *lastURI, tc.want)
			}
		})
	}
}

func TestLevelToolEscapesPathInput(t *testing.T) {
	session, lastURI := connect(t, http.StatusOK, `{}`)
	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_gender_by_level",
		Arguments: map[string]any{"standard": "industry", "level": "../crawler/runs", "id": 1,
			"from_date": "2026-01-01", "to_date": "2026-01-31"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(*lastURI, "/api/v1/industry/..%2Fcrawler%2Fruns/1/gender") {
		t.Fatalf("path input was not escaped: %q", *lastURI)
	}
}

func TestBackendErrorBecomesToolError(t *testing.T) {
	session, _ := connect(t, http.StatusBadRequest, `{"error":"Invalid level parameter"}`)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_total_job_count_by_level",
		Arguments: map[string]any{"standard": "occupation", "level": "bogus", "id": 1,
			"from_date": "2026-01-01", "to_date": "2026-01-31"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected a tool error")
	}
	if text := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, "Invalid level parameter") {
		t.Fatalf("backend error message not surfaced: %q", text)
	}
}

func TestMissingDatesRejectedBeforeBackendCall(t *testing.T) {
	session, lastURI := connect(t, http.StatusOK, `{}`)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_vacancy_total", Arguments: map[string]any{"from_date": "2026-01-01", "to_date": ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || *lastURI != "" {
		t.Fatalf("expected tool error without a backend call, got IsError=%v uri=%q", res.IsError, *lastURI)
	}
}
