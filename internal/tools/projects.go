package tools

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerWhoAmI(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("whoami",
		"Identify the TimePerformance account behind the configured API credentials. Use it first when unsure which tenant or user the server is talking to.")

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		who, err := client.WhoAmI()
		if err != nil {
			return errf("whoami failed: %v", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Authenticated as: %s\nInstance: %s", who, client.BaseURL())), nil
	})
}

func registerListProjects(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("list_projects",
		"List the TimePerformance projects visible to the credentials. Optionally filter by name fragment, include archived projects, or list project templates instead.",
		mcp.WithString("name_contains",
			mcp.Description("Case-insensitive filter on the project name or client name"),
		),
		mcp.WithBoolean("include_archived",
			mcp.Description("Include archived projects (default: false)"),
		),
		mcp.WithBoolean("templates",
			mcp.Description("List project templates instead of projects (default: false)"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		archived := req.GetBool("include_archived", false)
		filter := strings.ToLower(strings.TrimSpace(req.GetString("name_contains", "")))

		var (
			projects []tp.Project
			err      error
			title    = "Projects"
		)
		if req.GetBool("templates", false) {
			title = "Project templates"
			projects, err = client.ListProjectTemplates(archived)
		} else {
			projects, err = client.ListProjects(archived)
		}
		if err != nil {
			return errf("list projects failed: %v", err), nil
		}

		if filter != "" {
			kept := projects[:0:0]
			for _, p := range projects {
				if strings.Contains(strings.ToLower(p.Name), filter) || strings.Contains(strings.ToLower(p.Sponsor), filter) {
					kept = append(kept, p)
				}
			}
			projects = kept
			title = fmt.Sprintf("%s matching %q", title, filter)
		}
		sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })

		return mcp.NewToolResultText(FormatProjects(projects, title)), nil
	})
}

func registerGetProject(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_project",
		"Get one project by numeric id or exact name: state, client, type, priority, labels and description.",
		mcp.WithString("project",
			mcp.Description("Project numeric id or exact name"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveProject(client, req)
		if abort != nil {
			return abort, nil
		}
		project, err := client.GetProject(id)
		if err != nil {
			return errf("get project failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatProject(project)), nil
	})
}

func registerProjectReport(s *server.MCPServer, client *tp.Client) {
	kinds := make([]string, 0, len(tp.ProjectReportPath)+1)
	for k := range tp.ProjectReportPath {
		kinds = append(kinds, k)
	}
	kinds = append(kinds, "risks")
	sort.Strings(kinds)

	tool := newReadTool("get_project_report",
		"Get one of a project's reports: progress (earned value, RAG, cost/effort), roadmap, brief, baseline, config, workload plan, actuals time series, assignments, datasheets, risks, or the last modification timestamp. Reports other than 'risks' are returned as raw JSON.",
		mcp.WithString("project",
			mcp.Description("Project numeric id or exact name"),
			mcp.Required(),
		),
		mcp.WithString("kind",
			mcp.Description("Report to fetch: "+strings.Join(kinds, ", ")),
			mcp.Required(),
			mcp.Enum(kinds...),
		),
		mcp.WithBoolean("with_deliverables",
			mcp.Description("progress report only: include the deliverables breakdown (default: false)"),
		),
		mcp.WithBoolean("with_tasks",
			mcp.Description("roadmap only: include the tasks of each deliverable (default: false)"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveProject(client, req)
		if abort != nil {
			return abort, nil
		}
		kind := strings.ToLower(strings.TrimSpace(req.GetString("kind", "")))

		if kind == "risks" {
			risks, err := client.ListProjectRisks(id)
			if err != nil {
				return errf("list risks failed: %v", err), nil
			}
			return mcp.NewToolResultText(FormatRisks(risks, id)), nil
		}

		if _, ok := tp.ProjectReportPath[kind]; !ok {
			return errf("unknown kind %q — pick one of: %s", kind, strings.Join(kinds, ", ")), nil
		}

		params := url.Values{}
		if kind == "progress" && req.GetBool("with_deliverables", false) {
			params.Set("withDeliverables", "true")
		}
		if kind == "roadmap" && req.GetBool("with_tasks", false) {
			params.Set("tasks", "true")
		}

		raw, err := client.ProjectReport(id, kind, params)
		if err != nil {
			return errf("get %s report failed: %v", kind, err), nil
		}
		return mcp.NewToolResultText(FormatJSON(fmt.Sprintf("Project #%d — %s report", id, kind), raw)), nil
	})
}

func registerListProjectTeam(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("list_project_team",
		"List the members of a project's team with their project-manager right and skill profile.",
		mcp.WithString("project",
			mcp.Description("Project numeric id or exact name"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveProject(client, req)
		if abort != nil {
			return abort, nil
		}
		members, err := client.ListTeam(id)
		if err != nil {
			return errf("list team failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatTeam(members, id)), nil
	})
}

func registerCreateProject(s *server.MCPServer, client *tp.Client) {
	tool := newWriteTool("create_project",
		"Create a TimePerformance project. Back-office credentials require project_manager. Needs api:write (back-office credentials): user credentials are read-only.",
		false,
		mcp.WithString("name",
			mcp.Description("Project name"),
			mcp.Required(),
		),
		mcp.WithString("description", mcp.Description("Project description")),
		mcp.WithString("sponsor", mcp.Description("Client name")),
		mcp.WithString("project_type", mcp.Description("Project type")),
		mcp.WithString("external_id", mcp.Description("Id of the project in another system (UI: external reference)")),
		mcp.WithNumber("priority", mcp.Description("Priority from 1 (highest) to 5 (lowest)")),
		mcp.WithNumber("state", mcp.Description("ProjectState id (list_reference_data kind=projectstates)")),
		mcp.WithString("project_manager", mcp.Description("Project manager: user id, full name or email (required by the back-office API)")),
		mcp.WithNumber("phasing_depth", mcp.Description("0 = no phases, 1 = phases (default), 2 = phases and sub-phases")),
		mcp.WithNumber("template", mcp.Description("Id of the project template to clone")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}
		f := fields(req)
		name := f.str("name")
		if name == nil {
			return errf("name is required"), nil
		}

		payload := tp.ProjectWrite{
			Name:         name,
			Description:  f.str("description"),
			Sponsor:      f.str("sponsor"),
			ProjectType:  f.str("project_type"),
			ExternalID:   f.str("external_id"),
			Priority:     f.num("priority"),
			State:        f.num("state"),
			PhasingDepth: f.num("phasing_depth"),
			Template:     f.num("template"),
		}

		if ref := strings.TrimSpace(req.GetString("project_manager", "")); ref != "" {
			pmID, err := client.ResolveUserID(ref)
			if err != nil {
				return errf("project_manager: %v", err), nil
			}
			payload.ProjectManager = &pmID
		}

		summary := []string{
			summarize("Name", *name),
			summarize("Client", req.GetString("sponsor", "")),
			summarize("Type", req.GetString("project_type", "")),
			summarize("Description", req.GetString("description", "")),
			intSummary("Priority", payload.Priority, "%d"),
			intSummary("Project manager (user id)", payload.ProjectManager, "#%d"),
			intSummary("Cloned from template", payload.Template, "#%d"),
		}
		if abort := confirmWrite(ctx, s, "Create a new TimePerformance project", why, summary); abort != nil {
			return abort, nil
		}

		project, err := client.CreateProject(payload)
		if err != nil {
			return errf("create project failed: %v", err), nil
		}
		return mcp.NewToolResultText("Project created.\n\n" + FormatProject(project)), nil
	})
}

func registerUpdateProject(s *server.MCPServer, client *tp.Client) {
	tool := newWriteTool("update_project",
		"Update the writable fields of a project. Only the fields you pass are changed. Needs api:write (back-office credentials).",
		true,
		mcp.WithString("project",
			mcp.Description("Project numeric id or exact name"),
			mcp.Required(),
		),
		mcp.WithString("name", mcp.Description("New project name")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("sponsor", mcp.Description("New client name")),
		mcp.WithString("project_type", mcp.Description("New project type")),
		mcp.WithString("external_id", mcp.Description("New external reference")),
		mcp.WithNumber("priority", mcp.Description("New priority, 1 (highest) to 5 (lowest)")),
		mcp.WithNumber("state", mcp.Description("New ProjectState id (list_reference_data kind=projectstates)")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveProject(client, req)
		if abort != nil {
			return abort, nil
		}
		why, abortSummary := requireChangeSummary(req)
		if abortSummary != nil {
			return abortSummary, nil
		}

		f := fields(req)
		payload := tp.ProjectWrite{
			Name:        f.str("name"),
			Description: f.str("description"),
			Sponsor:     f.str("sponsor"),
			ProjectType: f.str("project_type"),
			ExternalID:  f.str("external_id"),
			Priority:    f.num("priority"),
			State:       f.num("state"),
		}
		if isEmpty(payload.Name, payload.Description, payload.Sponsor, payload.ProjectType, payload.ExternalID, payload.Priority, payload.State) {
			return errf("nothing to update: pass at least one field to change"), nil
		}

		summary := []string{
			fmt.Sprintf("Project: #%d", id),
			summarize("Name", req.GetString("name", "")),
			summarize("Client", req.GetString("sponsor", "")),
			summarize("Type", req.GetString("project_type", "")),
			summarize("Description", req.GetString("description", "")),
			intSummary("Priority", payload.Priority, "%d"),
			intSummary("State id", payload.State, "#%d"),
		}
		if abort := confirmWrite(ctx, s, "Update a TimePerformance project", why, summary); abort != nil {
			return abort, nil
		}

		project, err := client.UpdateProject(id, payload)
		if err != nil {
			return errf("update project failed: %v", err), nil
		}
		return mcp.NewToolResultText("Project updated.\n\n" + FormatProject(project)), nil
	})
}
