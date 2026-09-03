package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerManageProjectMember(s *server.MCPServer, client *tp.Client) {
	tool := newWriteTool("manage_project_member",
		"Add a user to a project team, change their project-manager right, or remove them from the team. Needs api:write (back-office credentials).",
		true,
		mcp.WithString("project",
			mcp.Description("Project numeric id or exact name"),
			mcp.Required(),
		),
		mcp.WithString("user",
			mcp.Description("User numeric id, full name, email, or \"me\""),
			mcp.Required(),
		),
		mcp.WithString("action",
			mcp.Description("add: add the user to the team; set_rights: toggle their project-manager right; remove: take them off the team"),
			mcp.Required(),
			mcp.Enum("add", "set_rights", "remove"),
		),
		mcp.WithBoolean("project_manager",
			mcp.Description("Grant the project-manager right (add and set_rights; default: false)"),
		),
		mcp.WithNumber("profile",
			mcp.Description("add only: skill profile id for this member (list_reference_data kind=profiles)"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, abort := resolveProject(client, req)
		if abort != nil {
			return abort, nil
		}
		userID, abortUser := resolveUser(client, req, "user")
		if abortUser != nil {
			return abortUser, nil
		}
		why, abortSummary := requireChangeSummary(req)
		if abortSummary != nil {
			return abortSummary, nil
		}

		action := strings.ToLower(strings.TrimSpace(req.GetString("action", "")))
		projectManager := req.GetBool("project_manager", false)
		profileID := req.GetInt("profile", 0)

		summary := []string{
			fmt.Sprintf("Project: #%d", projectID),
			fmt.Sprintf("User: #%d", userID),
			"Action: " + action,
		}
		if action != "remove" {
			summary = append(summary, fmt.Sprintf("Project manager right: %t", projectManager))
		}
		if action == "add" && profileID > 0 {
			summary = append(summary, fmt.Sprintf("Skill profile: #%d", profileID))
		}

		title := map[string]string{
			"add":        "Add a member to a TimePerformance project team",
			"set_rights": "Change a TimePerformance team member's rights",
			"remove":     "Remove a member from a TimePerformance project team",
		}[action]
		if title == "" {
			return errf("unknown action %q — use add, set_rights or remove", action), nil
		}
		if abort := confirmWrite(ctx, s, title, why, summary); abort != nil {
			return abort, nil
		}

		var err error
		switch action {
		case "add":
			err = client.AddTeamMember(projectID, userID, profileID, projectManager)
		case "set_rights":
			err = client.SetTeamMemberRights(projectID, userID, projectManager)
		case "remove":
			err = client.RemoveTeamMember(projectID, userID)
		}
		if err != nil {
			return errf("%s failed: %v", action, err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Done: %s user #%d on project #%d.", action, userID, projectID)), nil
	})
}

func registerDeleteElement(s *server.MCPServer, client *tp.Client) {
	kinds := make([]string, 0, len(tp.DeletePath))
	for k := range tp.DeletePath {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	tool := newWriteTool("delete_element",
		"Permanently delete a task, deliverable, phase, expense or datasheet. Deletion cannot be undone — prefer closing or cancelling an element when the history matters. Needs api:write (back-office credentials).",
		true,
		mcp.WithString("kind",
			mcp.Description("What to delete: "+strings.Join(kinds, ", ")),
			mcp.Required(),
			mcp.Enum(kinds...),
		),
		mcp.WithNumber("id",
			mcp.Description("Numeric id of the element to delete"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kind := strings.ToLower(strings.TrimSpace(req.GetString("kind", "")))
		if _, ok := tp.DeletePath[kind]; !ok {
			return errf("unknown kind %q — pick one of: %s", kind, strings.Join(kinds, ", ")), nil
		}
		id := req.GetInt("id", 0)
		if id == 0 {
			return errf("id is required"), nil
		}
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}

		// Name the target in the prompt so the human sees what disappears.
		label := fmt.Sprintf("%s #%d", kind, id)
		switch kind {
		case "task":
			if t, err := client.GetTask(id); err == nil {
				label = fmt.Sprintf("task #%d — %s (%s)", t.ID, t.Name, t.State)
			}
		case "deliverable":
			if d, err := client.GetDeliverable(id); err == nil {
				label = fmt.Sprintf("deliverable #%d — %s", d.ID, d.Name)
			}
		case "phase":
			if p, err := client.GetPhase(id); err == nil {
				label = fmt.Sprintf("phase #%d — %s", p.ID, p.Name)
			}
		case "expense":
			if e, err := client.GetExpense(id); err == nil {
				label = fmt.Sprintf("expense #%d — %s (%s)", e.ID, e.Name, amountLabel(e.Amount))
			}
		}

		summary := []string{
			"Target: " + label,
			"This deletion is permanent.",
		}
		if abort := confirmWrite(ctx, s, "Delete a TimePerformance element", why, summary); abort != nil {
			return abort, nil
		}

		if err := client.DeleteElement(kind, id); err != nil {
			return errf("delete %s #%d failed: %v", kind, id, err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Deleted %s.", label)), nil
	})
}
