package tools

import (
	"context"
	"fmt"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// taskFilterOptions are the listing filters shared by the project and user
// task tools.
func taskFilterOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithBoolean("not_closed",
			mcp.Description("Only tasks that are neither completed nor cancelled (default: false)"),
		),
		mcp.WithString("period_start",
			mcp.Description("Start of the period filter (ISO 8601 date, e.g. 2026-09-01)"),
		),
		mcp.WithString("period_end",
			mcp.Description("End of the period filter (ISO 8601 date, e.g. 2026-09-30)"),
		),
	}
}

func taskFilter(req mcp.CallToolRequest) tp.TaskFilter {
	return tp.TaskFilter{
		NotClosed:   req.GetBool("not_closed", false),
		PeriodStart: strings.TrimSpace(req.GetString("period_start", "")),
		PeriodEnd:   strings.TrimSpace(req.GetString("period_end", "")),
	}
}

func registerListProjectTasks(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithString("project",
			mcp.Description("Project numeric id or exact name"),
			mcp.Required(),
		),
	}, taskFilterOptions()...)

	tool := newReadTool("list_project_tasks",
		"List the tasks of a project, with their performer, state, due date and effort. Filter to open tasks with not_closed, or to a period with period_start/period_end.",
		opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveProject(client, req)
		if abort != nil {
			return abort, nil
		}
		tasks, err := client.ListProjectTasks(id, taskFilter(req))
		if err != nil {
			return errf("list tasks failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatTasks(tasks, fmt.Sprintf("Tasks of project #%d", id))), nil
	})
}

func registerGetTask(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_task",
		"Get one task by id: state, project, phase, deliverable, performer, dates, effort spent and remaining, description and check-list.",
		mcp.WithNumber("task_id",
			mcp.Description("Task numeric id"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID := req.GetInt("task_id", 0)
		if taskID == 0 {
			return errf("task_id is required"), nil
		}
		task, err := client.GetTask(taskID)
		if err != nil {
			return errf("get task failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatTask(task)), nil
	})
}

// taskWriteOptions are the writable task fields.
func taskWriteOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("description", mcp.Description("Task description")),
		mcp.WithString("due_date", mcp.Description("Due date (ISO 8601 date, e.g. 2026-09-30)")),
		mcp.WithString("performer", mcp.Description("User assigned to the task: id, full name, email or \"me\"")),
		mcp.WithNumber("deliverable", mcp.Description("Deliverable id the task contributes to (list_project_deliverables)")),
		mcp.WithNumber("task_type", mcp.Description("TaskDefinition id from the project methodology")),
		mcp.WithString("external_id", mcp.Description("Id of the task in another system (UI: external reference)")),
	}
}

// taskWrite builds the payload from the shared writable fields.
func taskWrite(client *tp.Client, req mcp.CallToolRequest) (tp.TaskWrite, *mcp.CallToolResult) {
	f := fields(req)
	payload := tp.TaskWrite{
		Name:        f.str("name"),
		Description: f.str("description"),
		DueDate:     f.str("due_date"),
		Goal:        f.num("deliverable"),
		TaskType:    f.num("task_type"),
		ExternalID:  f.str("external_id"),
	}
	if ref := strings.TrimSpace(req.GetString("performer", "")); ref != "" {
		userID, err := client.ResolveUserID(ref)
		if err != nil {
			return payload, errf("performer: %v", err)
		}
		payload.Performer = &userID
	}
	return payload, nil
}

func taskSummary(req mcp.CallToolRequest, payload tp.TaskWrite) []string {
	return []string{
		summarize("Name", req.GetString("name", "")),
		summarize("Description", req.GetString("description", "")),
		summarize("Due date", req.GetString("due_date", "")),
		intSummary("Performer (user id)", payload.Performer, "#%d"),
		intSummary("Deliverable", payload.Goal, "#%d"),
		intSummary("Task type", payload.TaskType, "#%d"),
	}
}

func registerCreateTask(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithString("project",
			mcp.Description("Project numeric id or exact name"),
			mcp.Required(),
		),
		mcp.WithString("name",
			mcp.Description("Task name (required unless task_type is given)"),
		),
	}, taskWriteOptions()...)

	tool := newWriteTool("create_task",
		"Create a task in a project. Needs api:write (back-office credentials): user credentials are read-only.",
		false, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, abort := resolveProject(client, req)
		if abort != nil {
			return abort, nil
		}
		why, abortSummary := requireChangeSummary(req)
		if abortSummary != nil {
			return abortSummary, nil
		}
		payload, abortPayload := taskWrite(client, req)
		if abortPayload != nil {
			return abortPayload, nil
		}
		if payload.Name == nil && payload.TaskType == nil {
			return errf("name is required unless task_type is provided"), nil
		}

		summary := append([]string{fmt.Sprintf("Project: #%d", projectID)}, taskSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Create a TimePerformance task", why, summary); abort != nil {
			return abort, nil
		}

		task, err := client.CreateTask(projectID, payload)
		if err != nil {
			return errf("create task failed: %v", err), nil
		}
		return mcp.NewToolResultText("Task created.\n\n" + FormatTask(task)), nil
	})
}

func registerUpdateTask(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithNumber("task_id",
			mcp.Description("Task numeric id"),
			mcp.Required(),
		),
		mcp.WithString("name", mcp.Description("New task name")),
	}, taskWriteOptions()...)

	tool := newWriteTool("update_task",
		"Update a task. Only the fields you pass are changed. Needs api:write (back-office credentials).",
		true, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID := req.GetInt("task_id", 0)
		if taskID == 0 {
			return errf("task_id is required"), nil
		}
		why, abortSummary := requireChangeSummary(req)
		if abortSummary != nil {
			return abortSummary, nil
		}
		payload, abortPayload := taskWrite(client, req)
		if abortPayload != nil {
			return abortPayload, nil
		}
		if isEmpty(payload.Name, payload.Description, payload.DueDate, payload.Performer, payload.Goal, payload.TaskType, payload.ExternalID) {
			return errf("nothing to update: pass at least one field to change"), nil
		}

		summary := append([]string{fmt.Sprintf("Task: #%d", taskID)}, taskSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Update a TimePerformance task", why, summary); abort != nil {
			return abort, nil
		}

		task, err := client.UpdateTask(taskID, payload)
		if err != nil {
			return errf("update task failed: %v", err), nil
		}
		return mcp.NewToolResultText("Task updated.\n\n" + FormatTask(task)), nil
	})
}
