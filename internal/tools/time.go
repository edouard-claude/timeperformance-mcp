package tools

import (
	"context"
	"fmt"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// timeOptions are the parameters shared by the timesheet and time report tools.
func timeOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("user",
			mcp.Description("User numeric id, full name, email, or \"me\""),
			mcp.Required(),
		),
		mcp.WithString("first_day",
			mcp.Description("First day of the period (ISO 8601 date, e.g. 2026-09-01)"),
			mcp.Required(),
		),
		mcp.WithString("last_day",
			mcp.Description("Last day of the period (ISO 8601 date, e.g. 2026-09-30)"),
			mcp.Required(),
		),
		mcp.WithNumber("hours_per_day",
			mcp.Description("Equalize the report so each worked day totals this many hours"),
		),
		mcp.WithNumber("half_day_threshold",
			mcp.Description("With hours_per_day: hours below which a day counts as a half-day (default: half of hours_per_day)"),
		),
	}
}

func timeParams(req mcp.CallToolRequest, firstDay, lastDay string) tp.TimeParams {
	return tp.TimeParams{
		FirstDay:         firstDay,
		LastDay:          lastDay,
		HoursPerDay:      req.GetFloat("hours_per_day", 0),
		HalfDayThreshold: req.GetFloat("half_day_threshold", 0),
		SortTasks:        req.GetBool("sort_tasks", false),
	}
}

func registerUserTimesheet(s *server.MCPServer, client *tp.Client) {
	opts := append(timeOptions(),
		mcp.WithBoolean("sort_tasks",
			mcp.Description("Sort tasks alphabetically within each day (default: false)"),
		),
	)

	tool := newReadTool("get_user_timesheet",
		"Get the timesheet of a user over a period: hours logged per project, per task and per day. This is the detailed view — use get_user_time_report for the per-activity totals.",
		opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveUser(client, req, "user")
		if abort != nil {
			return abort, nil
		}
		firstDay, lastDay, abortDates := requireDates(req)
		if abortDates != nil {
			return abortDates, nil
		}
		sheet, err := client.GetUserTimesheet(id, timeParams(req, firstDay, lastDay))
		if err != nil {
			return errf("get timesheet failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatTimesheet(sheet)), nil
	})
}

func registerUserTimeReport(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_user_time_report",
		"Get the activity report of a user over a period: hours logged per project and per non-project activity, without the task breakdown.",
		timeOptions()...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveUser(client, req, "user")
		if abort != nil {
			return abort, nil
		}
		firstDay, lastDay, abortDates := requireDates(req)
		if abortDates != nil {
			return abortDates, nil
		}
		report, err := client.GetUserTimeReport(id, timeParams(req, firstDay, lastDay))
		if err != nil {
			return errf("get time report failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatTimeReport(report)), nil
	})
}

func registerListUserTasks(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithString("user",
			mcp.Description("User numeric id, full name, email, or \"me\""),
			mcp.Required(),
		),
		mcp.WithString("scope",
			mcp.Description("'all' (default): every task assigned to the user; 'todo': the user's current to-do list"),
			mcp.Enum("all", "todo"),
		),
		mcp.WithString("project",
			mcp.Description("todo scope only: restrict the to-do list to this project (id or exact name)"),
		),
	}, taskFilterOptions()...)

	tool := newReadTool("list_user_tasks",
		"List the tasks assigned to a user across projects, or their current to-do list with scope='todo'.",
		opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveUser(client, req, "user")
		if abort != nil {
			return abort, nil
		}

		if strings.EqualFold(req.GetString("scope", "all"), "todo") {
			projectID := 0
			if ref := strings.TrimSpace(req.GetString("project", "")); ref != "" {
				resolved, err := client.ResolveProjectID(ref)
				if err != nil {
					return errf("%v", err), nil
				}
				projectID = resolved
			}
			tasks, err := client.GetUserTodoList(id, projectID)
			if err != nil {
				return errf("get to-do list failed: %v", err), nil
			}
			return mcp.NewToolResultText(FormatTasks(tasks, fmt.Sprintf("To-do list of user #%d", id))), nil
		}

		tasks, err := client.ListUserTasks(id, taskFilter(req))
		if err != nil {
			return errf("list user tasks failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatTasks(tasks, fmt.Sprintf("Tasks of user #%d", id))), nil
	})
}

func registerUserAssignments(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_user_assignments",
		"Get the half-day assignment schedule of a user over a period: which project or activity they are planned on, morning and afternoon.",
		mcp.WithString("user",
			mcp.Description("User numeric id, full name, email, or \"me\""),
			mcp.Required(),
		),
		mcp.WithString("first_day",
			mcp.Description("First day of the period (ISO 8601 date)"),
			mcp.Required(),
		),
		mcp.WithString("last_day",
			mcp.Description("Last day of the period (ISO 8601 date)"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveUser(client, req, "user")
		if abort != nil {
			return abort, nil
		}
		firstDay, lastDay, abortDates := requireDates(req)
		if abortDates != nil {
			return abortDates, nil
		}
		assignments, err := client.GetUserAssignments(id, firstDay, lastDay)
		if err != nil {
			return errf("get assignments failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatAssignments(assignments, id)), nil
	})
}
