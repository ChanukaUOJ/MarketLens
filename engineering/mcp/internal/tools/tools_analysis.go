package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"marketlens-mcp/internal/backend"
)

// occupationLevelInput is used for the occupation-only skill and employer tools.
type occupationLevelInput struct {
	Level    string `json:"level" jsonschema:"occupation hierarchy level, e.g. 'major-group'"`
	ID       uint   `json:"id" jsonschema:"the numeric id at that level"`
	FromDate string `json:"from_date" jsonschema:"start date, format YYYY-MM-DD"`
	ToDate   string `json:"to_date" jsonschema:"end date, format YYYY-MM-DD"`
}

type occupationLevelPageInput struct {
	Level    string `json:"level" jsonschema:"occupation hierarchy level, e.g. 'major-group'"`
	ID       uint   `json:"id" jsonschema:"the numeric id at that level"`
	FromDate string `json:"from_date" jsonschema:"start date, format YYYY-MM-DD"`
	ToDate   string `json:"to_date" jsonschema:"end date, format YYYY-MM-DD"`
	Limit    int    `json:"limit,omitempty" jsonschema:"max rows to return, default 20"`
	Offset   int    `json:"offset,omitempty" jsonschema:"rows to skip, default 0"`
}

func registerAnalysisTools(server *mcp.Server, api *backend.Client) {
	// Occupation skills (occupation only - no industry equivalent exists)

	registerOccupationTool(server, api, "get_top_15_skills_by_occupation",
		"Get the top 15 in-demand skills for a given occupation hierarchy level and id, within a date range.",
		"top-15-skills")

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_all_skills_by_occupation",
		Description: "Get all skills (paginated) for a given occupation hierarchy level and id, within a date range.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in occupationLevelPageInput) (*mcp.CallToolResult, any, error) {
		q, err := dateQuery(in.FromDate, in.ToDate)
		if err != nil {
			return nil, nil, err
		}
		out, err := api.Get(ctx, backend.Path("occupation", in.Level, idString(in.ID), "all-skills"), pageInput{Limit: in.Limit, Offset: in.Offset}.addTo(q))
		if err != nil {
			return nil, nil, err
		}
		return nil, wrapIfList(out), nil
	})

	registerOccupationTool(server, api, "get_top_hiring_employers_by_occupation",
		"Get the top hiring employers for a given occupation hierarchy level and id, within a date range.",
		"top-hiring-employers")

	// National, not level-scoped

	registerDateRangeTool(server, api, "get_vacancy_trend",
		"Get the national vacancy trend over a date range. Automatically buckets weekly "+
			"for ranges under 60 days, or monthly for longer ranges - matching GET /vacancy-trend.",
		"vacancy-trend")
	registerDateRangeTool(server, api, "get_vacancy_total",
		"Get the total national vacancy count for a date range.",
		"vacancy-total")
	registerDateRangeTool(server, api, "get_occupations_by_date_range",
		"Get vacancy counts grouped by occupation major group for a date range.",
		"occupations", "by-date-range")
	registerDateRangeTool(server, api, "get_industries_by_date_range",
		"Get vacancy counts grouped by industry sector for a date range.",
		"industries", "by-date-range")
}

// registerOccupationTool registers a tool backed by
// GET /occupation/:level/:id/{metric}?from-date=&to-date=.
func registerOccupationTool(server *mcp.Server, api *backend.Client, name, description, metric string) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, _ *mcp.CallToolRequest, in occupationLevelInput) (*mcp.CallToolResult, any, error) {
			q, err := dateQuery(in.FromDate, in.ToDate)
			if err != nil {
				return nil, nil, err
			}
			out, err := api.Get(ctx, backend.Path("occupation", in.Level, idString(in.ID), metric), q)
			if err != nil {
				return nil, nil, err
			}
			return nil, wrapIfList(out), nil
		},
	)
}

// registerDateRangeTool registers a tool backed by GET /{path...}?from-date=&to-date=.
func registerDateRangeTool(server *mcp.Server, api *backend.Client, name, description string, path ...string) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, _ *mcp.CallToolRequest, in dateRangeInput) (*mcp.CallToolResult, any, error) {
			q, err := dateQuery(in.FromDate, in.ToDate)
			if err != nil {
				return nil, nil, err
			}
			out, err := api.Get(ctx, backend.Path(path...), q)
			if err != nil {
				return nil, nil, err
			}
			return nil, wrapIfList(out), nil
		},
	)
}
