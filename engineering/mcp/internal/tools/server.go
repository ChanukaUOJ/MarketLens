package tools

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"marketlens-mcp/internal/backend"
)

// SubmitVacanciesScope is the OAuth scope required by submit_newspaper_vacancies.
const SubmitVacanciesScope = "submit-newspaper-vacancies"

// New builds the MCP server. Read-only tools are thin wrappers over the
// backend's public GET endpoints; the manual upload tool posts to the crawler.
func New(api *backend.Client, crawlerURL string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "marketlens-mcp",
		Version: "v1.0.0",
	}, nil)

	registerLookupTools(server, api)
	registerHierarchyTools(server, api)
	registerAnalysisTools(server, api)
	registerManualUploadTools(server, crawlerURL)

	return server
}

func wrapIfList(v any) any {
	if v == nil {
		return v
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		return map[string]any{"items": v}
	case reflect.Map, reflect.Struct, reflect.Ptr:
		return v
	default:
		return map[string]any{"value": v}
	}
}

// emptyInput is used for tools that take no parameters at all.
type emptyInput struct{}

// registerListTool registers a tool that takes no input and returns the
// response of GET path.
func registerListTool(server *mcp.Server, api *backend.Client, name, description, path string) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
			out, err := api.Get(ctx, backend.Path(path), nil)
			if err != nil {
				return nil, nil, err
			}
			return nil, wrapIfList(out), nil
		},
	)
}

// idInput is used for tools scoped to a single numeric id (e.g. an
// industry_sector id, a major_group id).
type idInput struct {
	ID uint `json:"id" jsonschema:"the numeric id to look up"`
}

// registerIDTool registers a tool that takes a single "id" parameter and
// returns the response of GET path/{id}.
func registerIDTool(server *mcp.Server, api *backend.Client, name, description, path string) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
			out, err := api.Get(ctx, backend.Path(path, idString(in.ID)), nil)
			if err != nil {
				return nil, nil, err
			}
			return nil, wrapIfList(out), nil
		},
	)
}

// pageInput is used for paginated list tools.
type pageInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"max rows to return, default 20"`
	Offset int `json:"offset,omitempty" jsonschema:"rows to skip, default 0"`
}

func (p pageInput) addTo(q url.Values) url.Values {
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		q.Set("offset", strconv.Itoa(p.Offset))
	}
	return q
}

// registerPagedListTool registers a paginated list tool backed by GET path?limit=&offset=.
func registerPagedListTool(server *mcp.Server, api *backend.Client, name, description, path string) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, _ *mcp.CallToolRequest, in pageInput) (*mcp.CallToolResult, any, error) {
			out, err := api.Get(ctx, backend.Path(path), in.addTo(url.Values{}))
			if err != nil {
				return nil, nil, err
			}
			return nil, wrapIfList(out), nil
		},
	)
}

// dateRangeInput is used for tools scoped to a from/to date window.
type dateRangeInput struct {
	FromDate string `json:"from_date" jsonschema:"start date, format YYYY-MM-DD"`
	ToDate   string `json:"to_date" jsonschema:"end date, format YYYY-MM-DD"`
}

// dateQuery builds the from-date/to-date query the backend expects. Format and
// ordering are validated by the backend.
func dateQuery(fromDate, toDate string) (url.Values, error) {
	if fromDate == "" || toDate == "" {
		return nil, fmt.Errorf("from_date and to_date are both required (format YYYY-MM-DD)")
	}
	return url.Values{"from-date": {fromDate}, "to-date": {toDate}}, nil
}

func idString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func registerLookupTools(server *mcp.Server, api *backend.Client) {
	registerListTool(server, api, "get_experiences", "List all experience levels.", "experiences")
	registerListTool(server, api, "get_provinces", "List all Sri Lankan provinces used for geo-tagging job posts.", "provinces")
	registerListTool(server, api, "get_job_types", "List all job types (Full Time, Part Time, Contract, Internship).", "job-types")
	registerListTool(server, api, "get_employment_sectors", "List all employment sectors (Government, Private, NGO, etc.).", "employment-sectors")
	registerListTool(server, api, "get_education_levels", "List all education level categories.", "education-levels")
	registerListTool(server, api, "get_formalities", "List all formality categories (Formal / Informal sector).", "formalities")
	registerListTool(server, api, "get_genders", "List all gender categories used in job postings.", "genders")
	registerListTool(server, api, "get_vocational_educations", "List all vocational education (NVQ) levels.", "vocational-educations")
}
