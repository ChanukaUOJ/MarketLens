package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"marketlens-mcp/internal/backend"
)

func registerHierarchyTools(server *mcp.Server, api *backend.Client) {
	registerListTool(server, api, "get_major_groups", "List all occupation major groups (top level of the SLSO occupation hierarchy).", "major-groups")
	registerIDTool(server, api, "get_major_group", "Get a single occupation major group by id.", "major-groups")

	registerListTool(server, api, "get_sub_major_groups", "List all occupation sub major groups.", "sub-major-groups")
	registerIDTool(server, api, "get_sub_major_group", "Get a single occupation sub major group by id.", "sub-major-groups")

	registerListTool(server, api, "get_minor_groups", "List all occupation minor groups.", "minor-groups")
	registerIDTool(server, api, "get_minor_group", "Get a single occupation minor group by id.", "minor-groups")

	registerListTool(server, api, "get_unit_groups", "List all occupation unit groups.", "unit-groups")
	registerIDTool(server, api, "get_unit_group", "Get a single occupation unit group by id.", "unit-groups")

	registerPagedListTool(server, api, "get_occupation_groups", "List occupation groups (leaf level of the SLSO hierarchy), paginated.", "occupation-groups")
	registerIDTool(server, api, "get_occupation_group", "Get a single occupation group by id.", "occupation-groups")

	registerListTool(server, api, "get_industry_sectors", "List all industry sectors (top level of the SLSIC industry hierarchy).", "industry-sectors")
	registerIDTool(server, api, "get_industry_sector", "Get a single industry sector by id.", "industry-sectors")

	registerListTool(server, api, "get_industry_divisions", "List all industry divisions.", "industry-divisions")
	registerIDTool(server, api, "get_industry_division", "Get a single industry division by id.", "industry-divisions")

	registerListTool(server, api, "get_industry_groups", "List all industry groups.", "industry-groups")
	registerIDTool(server, api, "get_industry_group", "Get a single industry group by id.", "industry-groups")

	registerListTool(server, api, "get_industry_classes", "List all industry classes.", "industry-classes")
	registerIDTool(server, api, "get_industry_class", "Get a single industry class by id.", "industry-classes")

	registerPagedListTool(server, api, "get_industry_subclasses", "List industry subclasses (leaf level of the SLSIC hierarchy), paginated.", "industry-subclasses")
	registerIDTool(server, api, "get_industry_subclass", "Get a single industry subclass by id.", "industry-subclasses")

	registerLevelTool(server, api, "get_hierarchy_children",
		"Get the immediate child-level entities under a given occupation/industry hierarchy "+
			"level and id, each with its aggregated job count for a date range. Fails for leaf levels "+
			"('occupation-group', 'industry-subclass'), which have no children.",
		"children")
	registerLevelTool(server, api, "get_total_job_count_by_level",
		"Get the total vacancy count for a given occupation/industry hierarchy level and id, within a date range.",
		"total-job-count")
	registerLevelTool(server, api, "get_employment_sector_by_level",
		"Get employment sector breakdown for a given occupation/industry hierarchy level and id, within a date range.",
		"employment-sector")
	registerLevelTool(server, api, "get_experience_by_level",
		"Get experience-level breakdown for a given occupation/industry hierarchy level and id, within a date range.",
		"experience")
	registerLevelTool(server, api, "get_province_by_level",
		"Get province breakdown for a given occupation/industry hierarchy level and id, within a date range.",
		"province")
	registerLevelTool(server, api, "get_education_by_level",
		"Get education level breakdown for a given occupation/industry hierarchy level and id, within a date range.",
		"education")
	registerLevelTool(server, api, "get_formality_by_level",
		"Get formal/informal breakdown for a given occupation/industry hierarchy level and id, within a date range.",
		"formality")
	registerLevelTool(server, api, "get_gender_by_level",
		"Get gender breakdown for a given occupation/industry hierarchy level and id, within a date range.",
		"gender")
	registerLevelTool(server, api, "get_vocational_education_by_level",
		"Get vocational education (NVQ) breakdown for a given occupation/industry hierarchy level and id, within a date range.",
		"vocational-education")
	registerLevelTool(server, api, "get_remote_onsite_by_level",
		"Get remote vs on-site job counts for a given occupation/industry hierarchy level and id, within a date range.",
		"remote-onsite-hybrid")
	registerLevelTool(server, api, "get_job_type_by_level",
		"Get job type breakdown for a given occupation/industry hierarchy level and id, within a date range.",
		"job-type")
}

// levelInput mirrors the /:standard/:level/:id/... path params, plus the
// from/to date query params every breakdown endpoint requires.
type levelInput struct {
	Standard string `json:"standard" jsonschema:"'occupation' or 'industry'"`
	Level    string `json:"level" jsonschema:"the hierarchy level name, e.g. 'major-group' or 'industry-sector'"`
	ID       uint   `json:"id" jsonschema:"the numeric id at that level"`
	FromDate string `json:"from_date" jsonschema:"start date, format YYYY-MM-DD"`
	ToDate   string `json:"to_date" jsonschema:"end date, format YYYY-MM-DD"`
}

// registerLevelTool registers a tool backed by
// GET /:standard/:level/:id/{metric}?from-date=&to-date=.
func registerLevelTool(server *mcp.Server, api *backend.Client, name, description, metric string) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, _ *mcp.CallToolRequest, in levelInput) (*mcp.CallToolResult, any, error) {
			if in.Standard != "occupation" && in.Standard != "industry" {
				return nil, nil, fmt.Errorf("invalid standard %q, must be 'occupation' or 'industry'", in.Standard)
			}
			q, err := dateQuery(in.FromDate, in.ToDate)
			if err != nil {
				return nil, nil, err
			}
			out, err := api.Get(ctx, backend.Path(in.Standard, in.Level, idString(in.ID), metric), q)
			if err != nil {
				return nil, nil, err
			}
			return nil, wrapIfList(out), nil
		},
	)
}
