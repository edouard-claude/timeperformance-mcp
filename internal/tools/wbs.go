package tools

import (
	"context"
	"fmt"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// wbsStates are the values accepted by the state field of deliverables and phases.
var wbsStates = []string{"draft", "ready", "open", "closed", "cancelled"}

// progressModes are the progress tracking modes of a deliverable.
var progressModes = []string{"MANUAL", "TASK_RTD", "TASK_COUNT", "SCHEDULE", "WORKLOAD", "WEIGHTED_SUM"}

// wbsWriteOptions are the writable fields shared by deliverables and phases.
func wbsWriteOptions(deliverable bool) []mcp.ToolOption {
	opts := []mcp.ToolOption{
		mcp.WithString("description", mcp.Description("Description")),
		mcp.WithString("state",
			mcp.Description("State: "+strings.Join(wbsStates, ", ")),
			mcp.Enum(wbsStates...),
		),
		mcp.WithString("first_day", mcp.Description("Planned start date (ISO 8601 date). Omit to let the schedule compute it")),
		mcp.WithString("last_day", mcp.Description("Planned end date (ISO 8601 date). Omit to let the schedule compute it")),
		mcp.WithBoolean("milestone", mcp.Description("Mark as a milestone")),
		mcp.WithBoolean("locked_schedule", mcp.Description("Lock the schedule dates (no auto-computation)")),
		mcp.WithString("external_id", mcp.Description("Id in another system (UI: external reference)")),
	}
	if deliverable {
		opts = append(opts,
			mcp.WithString("progress_mode",
				mcp.Description("How progress is tracked: "+strings.Join(progressModes, ", ")),
				mcp.Enum(progressModes...),
			),
			mcp.WithString("acceptance", mcp.Description("Acceptance criteria")),
			mcp.WithNumber("phase", mcp.Description("Phase id the deliverable is planned for (list_project_phases)")),
		)
	}
	return opts
}

func wbsWrite(req mcp.CallToolRequest, deliverable bool) tp.WBSWrite {
	f := fields(req)
	w := tp.WBSWrite{
		Name:           f.str("name"),
		Description:    f.str("description"),
		State:          f.str("state"),
		FirstDay:       f.str("first_day"),
		LastDay:        f.str("last_day"),
		Milestone:      f.boolean("milestone"),
		LockedSchedule: f.boolean("locked_schedule"),
		ExternalID:     f.str("external_id"),
	}
	if deliverable {
		w.ProgressMode = f.str("progress_mode")
		w.Acceptance = f.str("acceptance")
		w.Iteration = f.num("phase")
	}
	return w
}

func wbsEmpty(w tp.WBSWrite) bool {
	return isEmpty(w.Name, w.Description, w.State, w.FirstDay, w.LastDay, w.Milestone,
		w.LockedSchedule, w.ExternalID, w.ProgressMode, w.Acceptance, w.Iteration)
}

func wbsSummary(req mcp.CallToolRequest, w tp.WBSWrite) []string {
	summary := []string{
		summarize("Name", req.GetString("name", "")),
		summarize("Description", req.GetString("description", "")),
		summarize("State", req.GetString("state", "")),
		summarize("Planned start", req.GetString("first_day", "")),
		summarize("Planned end", req.GetString("last_day", "")),
		summarize("Acceptance criteria", req.GetString("acceptance", "")),
		summarize("Progress mode", req.GetString("progress_mode", "")),
		intSummary("Phase", w.Iteration, "#%d"),
	}
	if w.Milestone != nil {
		summary = append(summary, fmt.Sprintf("Milestone: %t", *w.Milestone))
	}
	if w.LockedSchedule != nil {
		summary = append(summary, fmt.Sprintf("Locked schedule: %t", *w.LockedSchedule))
	}
	return summary
}

// --- Deliverables ---

func registerListDeliverables(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("list_project_deliverables",
		"List the deliverables (WBS goals) of a project as a tree, with state, milestone flag, planned dates and progress mode.",
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
		items, err := client.ListDeliverables(id)
		if err != nil {
			return errf("list deliverables failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatWBSTree(items, fmt.Sprintf("Deliverables of project #%d", id))), nil
	})
}

func registerGetDeliverable(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_deliverable",
		"Get one deliverable by id: state, planned dates, progress mode, acceptance criteria and children.",
		mcp.WithNumber("deliverable_id",
			mcp.Description("Deliverable numeric id"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetInt("deliverable_id", 0)
		if id == 0 {
			return errf("deliverable_id is required"), nil
		}
		item, err := client.GetDeliverable(id)
		if err != nil {
			return errf("get deliverable failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatWBSElement(item, "Deliverable")), nil
	})
}

func registerCreateDeliverable(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithString("project", mcp.Description("Project numeric id or exact name (for a top-level deliverable)")),
		mcp.WithNumber("parent_deliverable", mcp.Description("Parent deliverable id (creates a sub-deliverable instead)")),
		mcp.WithString("name", mcp.Description("Deliverable name"), mcp.Required()),
	}, wbsWriteOptions(true)...)

	tool := newWriteTool("create_deliverable",
		"Create a deliverable in a project, or a sub-deliverable under an existing one. Pass either project or parent_deliverable. Needs api:write (back-office credentials).",
		false, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}
		payload := wbsWrite(req, true)
		if payload.Name == nil {
			return errf("name is required"), nil
		}

		parentID := req.GetInt("parent_deliverable", 0)
		if parentID == 0 && strings.TrimSpace(req.GetString("project", "")) == "" {
			return errf("pass project (top-level deliverable) or parent_deliverable (sub-deliverable)"), nil
		}

		if parentID > 0 {
			summary := append([]string{fmt.Sprintf("Parent deliverable: #%d", parentID)}, wbsSummary(req, payload)...)
			if abort := confirmWrite(ctx, s, "Create a TimePerformance sub-deliverable", why, summary); abort != nil {
				return abort, nil
			}
			item, err := client.CreateSubDeliverable(parentID, payload)
			if err != nil {
				return errf("create sub-deliverable failed: %v", err), nil
			}
			return mcp.NewToolResultText("Sub-deliverable created.\n\n" + FormatWBSElement(item, "Deliverable")), nil
		}

		projectID, abortProject := resolveProject(client, req)
		if abortProject != nil {
			return abortProject, nil
		}
		summary := append([]string{fmt.Sprintf("Project: #%d", projectID)}, wbsSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Create a TimePerformance deliverable", why, summary); abort != nil {
			return abort, nil
		}
		item, err := client.CreateDeliverable(projectID, payload)
		if err != nil {
			return errf("create deliverable failed: %v", err), nil
		}
		return mcp.NewToolResultText("Deliverable created.\n\n" + FormatWBSElement(item, "Deliverable")), nil
	})
}

func registerUpdateDeliverable(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithNumber("deliverable_id", mcp.Description("Deliverable numeric id"), mcp.Required()),
		mcp.WithString("name", mcp.Description("New name")),
	}, wbsWriteOptions(true)...)

	tool := newWriteTool("update_deliverable",
		"Update a deliverable. Only the fields you pass are changed. Needs api:write (back-office credentials).",
		true, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetInt("deliverable_id", 0)
		if id == 0 {
			return errf("deliverable_id is required"), nil
		}
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}
		payload := wbsWrite(req, true)
		if wbsEmpty(payload) {
			return errf("nothing to update: pass at least one field to change"), nil
		}

		summary := append([]string{fmt.Sprintf("Deliverable: #%d", id)}, wbsSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Update a TimePerformance deliverable", why, summary); abort != nil {
			return abort, nil
		}

		item, err := client.UpdateDeliverable(id, payload)
		if err != nil {
			return errf("update deliverable failed: %v", err), nil
		}
		return mcp.NewToolResultText("Deliverable updated.\n\n" + FormatWBSElement(item, "Deliverable")), nil
	})
}

// --- Phases ---

func registerListPhases(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("list_project_phases",
		"List the phases of a project as a tree, with state, milestone flag and planned dates.",
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
		items, err := client.ListPhases(id)
		if err != nil {
			return errf("list phases failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatWBSTree(items, fmt.Sprintf("Phases of project #%d", id))), nil
	})
}

func registerGetPhase(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_phase",
		"Get one phase by id: state, planned dates, milestone flag and sub-phases.",
		mcp.WithNumber("phase_id",
			mcp.Description("Phase numeric id"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetInt("phase_id", 0)
		if id == 0 {
			return errf("phase_id is required"), nil
		}
		item, err := client.GetPhase(id)
		if err != nil {
			return errf("get phase failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatWBSElement(item, "Phase")), nil
	})
}

func registerCreatePhase(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithString("project", mcp.Description("Project numeric id or exact name (for a top-level phase)")),
		mcp.WithNumber("parent_phase", mcp.Description("Parent phase id (creates a sub-phase instead)")),
		mcp.WithString("name", mcp.Description("Phase name"), mcp.Required()),
	}, wbsWriteOptions(false)...)

	tool := newWriteTool("create_phase",
		"Create a phase in a project, or a sub-phase under an existing one. Pass either project or parent_phase. Needs api:write (back-office credentials).",
		false, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}
		payload := wbsWrite(req, false)
		if payload.Name == nil {
			return errf("name is required"), nil
		}

		parentID := req.GetInt("parent_phase", 0)
		if parentID == 0 && strings.TrimSpace(req.GetString("project", "")) == "" {
			return errf("pass project (top-level phase) or parent_phase (sub-phase)"), nil
		}

		if parentID > 0 {
			summary := append([]string{fmt.Sprintf("Parent phase: #%d", parentID)}, wbsSummary(req, payload)...)
			if abort := confirmWrite(ctx, s, "Create a TimePerformance sub-phase", why, summary); abort != nil {
				return abort, nil
			}
			item, err := client.CreateSubPhase(parentID, payload)
			if err != nil {
				return errf("create sub-phase failed: %v", err), nil
			}
			return mcp.NewToolResultText("Sub-phase created.\n\n" + FormatWBSElement(item, "Phase")), nil
		}

		projectID, abortProject := resolveProject(client, req)
		if abortProject != nil {
			return abortProject, nil
		}
		summary := append([]string{fmt.Sprintf("Project: #%d", projectID)}, wbsSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Create a TimePerformance phase", why, summary); abort != nil {
			return abort, nil
		}
		item, err := client.CreatePhase(projectID, payload)
		if err != nil {
			return errf("create phase failed: %v", err), nil
		}
		return mcp.NewToolResultText("Phase created.\n\n" + FormatWBSElement(item, "Phase")), nil
	})
}

func registerUpdatePhase(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithNumber("phase_id", mcp.Description("Phase numeric id"), mcp.Required()),
		mcp.WithString("name", mcp.Description("New name")),
	}, wbsWriteOptions(false)...)

	tool := newWriteTool("update_phase",
		"Update a phase. Only the fields you pass are changed. Needs api:write (back-office credentials).",
		true, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetInt("phase_id", 0)
		if id == 0 {
			return errf("phase_id is required"), nil
		}
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}
		payload := wbsWrite(req, false)
		if wbsEmpty(payload) {
			return errf("nothing to update: pass at least one field to change"), nil
		}

		summary := append([]string{fmt.Sprintf("Phase: #%d", id)}, wbsSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Update a TimePerformance phase", why, summary); abort != nil {
			return abort, nil
		}

		item, err := client.UpdatePhase(id, payload)
		if err != nil {
			return errf("update phase failed: %v", err), nil
		}
		return mcp.NewToolResultText("Phase updated.\n\n" + FormatWBSElement(item, "Phase")), nil
	})
}
