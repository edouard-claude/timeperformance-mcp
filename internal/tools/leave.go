package tools

import (
	"context"
	"fmt"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// maxListedHalfDays caps how many half-days are spelled out in a preview.
const maxListedHalfDays = 20

// LeaveSummary renders the confirmation lines of a leave change.
func LeaveSummary(plan tp.LeavePlan, activity tp.ReferenceItem, userID int, remove bool) []string {
	action := fmt.Sprintf("Add leave: #%d %s", activity.ID, activity.Name)
	if remove {
		action = "Remove leave"
		if activity.ID != 0 {
			action += fmt.Sprintf(": #%d %s only", activity.ID, activity.Name)
		}
	}
	summary := []string{
		fmt.Sprintf("User: #%d", userID),
		action,
		fmt.Sprintf("Sync window: %s → %s (every unavailability in it is rewritten)", plan.Sync.FirstDay, plan.Sync.LastDay),
		fmt.Sprintf("Half-days added: %d%s", len(plan.Added), listHalfDays(plan.Added)),
		fmt.Sprintf("Half-days removed: %d%s", len(plan.Removed), listHalfDays(plan.Removed)),
		fmt.Sprintf("Other unavailabilities kept in the window: %d half-day(s)", plan.Kept),
		"Conflict mode: " + plan.Sync.SyncMode,
	}
	return summary
}

func listHalfDays(days []string) string {
	if len(days) == 0 {
		return ""
	}
	if len(days) > maxListedHalfDays {
		return fmt.Sprintf(" (%s … %s)", days[0], days[len(days)-1])
	}
	return " (" + strings.Join(days, ", ") + ")"
}

// FormatSyncResult renders the answer of syncUnavailabilities.
func FormatSyncResult(res *tp.SyncResult, mode string) string {
	var b strings.Builder
	if res.Success {
		fmt.Fprintf(&b, "Leave saved: %d half-day assignment(s) updated.", res.Updated)
	} else {
		b.WriteString("Nothing was saved: the leave conflicts with existing assignments (ALL_OR_NOTHING).")
	}
	if len(res.Conflicts) > 0 {
		fmt.Fprintf(&b, "\n\nConflicting half-days (%d): %s", len(res.Conflicts), strings.Join(res.Conflicts, ", "))
		switch {
		case !res.Success:
			b.WriteString("\n\nRetry with sync_mode FORCE_OVERRIDE to replace those assignments, or NON_CONFLICT_ONLY to skip them.")
		case mode == tp.SyncNonConflictOnly:
			b.WriteString("\n\nThose half-days were skipped.")
		}
	}
	return b.String()
}

// NoLeaveChange is the message returned when the plan changes nothing.
const NoLeaveChange = "Nothing to change: the schedule already matches this request."

func registerSetUserLeave(s *server.MCPServer, client *tp.Client) {
	tool := newWriteTool("set_user_leave",
		"Add or remove a leave (any unavailability: paid leave, sick leave, holiday…) on a user's half-day schedule. "+
			"The API only offers a whole synchronization over a period, so this tool reads the current schedule first and keeps every other unavailability. "+
			"Leave types come from list_reference_data kind=npactivities (items flagged unavailable). Weekends are skipped unless include_weekends is set. "+
			"Needs api:write (back-office credentials).",
		true,
		mcp.WithString("user",
			mcp.Description("User numeric id, full name, email, or \"me\""),
			mcp.Required(),
		),
		mcp.WithString("action",
			mcp.Description("add (default): plan the leave on these half-days, replacing any other unavailability there; remove: clear the unavailability on these half-days"),
			mcp.Enum("add", "remove"),
		),
		mcp.WithString("leave_type",
			mcp.Description("Unavailability id or name (e.g. \"Congés payés\"). Required for add; on remove, restricts the removal to this type"),
		),
		mcp.WithString("first_day",
			mcp.Description("First day of the leave (ISO 8601 date)"),
			mcp.Required(),
		),
		mcp.WithString("last_day",
			mcp.Description("Last day of the leave (ISO 8601 date, inclusive)"),
			mcp.Required(),
		),
		mcp.WithString("first_half",
			mcp.Description("AM (default) or PM: PM starts the leave at noon on first_day"),
			mcp.Enum("AM", "PM"),
		),
		mcp.WithString("last_half",
			mcp.Description("PM (default) or AM: AM ends the leave at noon on last_day"),
			mcp.Enum("AM", "PM"),
		),
		mcp.WithBoolean("include_weekends",
			mcp.Description("Also plan Saturdays and Sundays (default: false)"),
		),
		mcp.WithString("sync_mode",
			mcp.Description("What to do when a half-day is already planned on a project or activity: ALL_OR_NOTHING (default, save nothing and list the conflicts), NON_CONFLICT_ONLY (skip those half-days), FORCE_OVERRIDE (replace them)"),
			mcp.Enum(tp.SyncModes...),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, abort := resolveUser(client, req, "user")
		if abort != nil {
			return abort, nil
		}
		firstDay, lastDay, abortDates := requireDates(req)
		if abortDates != nil {
			return abortDates, nil
		}
		action := strings.ToLower(strings.TrimSpace(req.GetString("action", "add")))
		if action != "add" && action != "remove" {
			return errf("unknown action %q — use add or remove", action), nil
		}
		why, abortSummary := requireChangeSummary(req)
		if abortSummary != nil {
			return abortSummary, nil
		}

		remove := action == "remove"
		plan, activity, err := client.PlanUserLeave(tp.LeaveRequest{
			UserID:    userID,
			Activity:  req.GetString("leave_type", ""),
			FirstDay:  firstDay,
			FirstHalf: req.GetString("first_half", ""),
			LastDay:   lastDay,
			LastHalf:  req.GetString("last_half", ""),
			Weekends:  req.GetBool("include_weekends", false),
			Remove:    remove,
			SyncMode:  req.GetString("sync_mode", ""),
		})
		if err != nil {
			return errf("%v", err), nil
		}
		if len(plan.Added) == 0 && len(plan.Removed) == 0 {
			return mcp.NewToolResultText(NoLeaveChange), nil
		}

		title := "Add a leave in TimePerformance"
		if remove {
			title = "Remove a leave in TimePerformance"
		}
		if abort := confirmWrite(ctx, s, title, why, LeaveSummary(plan, activity, userID, remove)); abort != nil {
			return abort, nil
		}

		res, err := client.SyncUserUnavailabilities(userID, plan.Sync)
		if err != nil {
			return errf("sync unavailabilities failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatSyncResult(res, plan.Sync.SyncMode)), nil
	})
}
