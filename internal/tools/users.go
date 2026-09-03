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

func registerListUsers(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("list_users",
		"List the TimePerformance user directory, with email, skill profile and organizational unit. Filter with query to find someone by name or email.",
		mcp.WithString("query",
			mcp.Description("Case-insensitive filter on name or email"),
		),
		mcp.WithBoolean("include_archived",
			mcp.Description("Include archived accounts (default: false)"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		users, err := client.ListUsers()
		if err != nil {
			return errf("list users failed: %v", err), nil
		}

		query := strings.ToLower(strings.TrimSpace(req.GetString("query", "")))
		includeArchived := req.GetBool("include_archived", false)
		kept := users[:0:0]
		for _, u := range users {
			if u.Archived && !includeArchived {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(u.Name), query) && !strings.Contains(strings.ToLower(u.Email), query) {
				continue
			}
			kept = append(kept, u)
		}
		sort.Slice(kept, func(i, j int) bool { return kept[i].Name < kept[j].Name })

		title := "Users"
		if query != "" {
			title = fmt.Sprintf("Users matching %q", query)
		}
		return mcp.NewToolResultText(FormatUsers(kept, title)), nil
	})
}

func registerGetUser(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_user",
		"Get one user account: email, availability period, organizational unit, skill profile and application rights.",
		mcp.WithString("user",
			mcp.Description("User numeric id, full name, email, or \"me\" for the authenticated account"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveUser(client, req, "user")
		if abort != nil {
			return abort, nil
		}
		user, err := client.GetUser(id)
		if err != nil {
			return errf("get user failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatUser(user)), nil
	})
}

// userWriteOptions are the writable fields of a user account.
func userWriteOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("email", mcp.Description("Email address")),
		mcp.WithString("availability_start", mcp.Description("Start of the availability period (ISO 8601 date)")),
		mcp.WithString("availability_end", mcp.Description("End of the availability period (ISO 8601 date)")),
		mcp.WithNumber("obs", mcp.Description("Organizational unit id (list_reference_data kind=obs)")),
		mcp.WithNumber("profile", mcp.Description("Skill profile id (list_reference_data kind=profiles)")),
		mcp.WithString("locale", mcp.Description("Locale code, e.g. fr or en")),
		mcp.WithString("gender", mcp.Description("Gender as accepted by the API")),
		mcp.WithString("sso_uid", mcp.Description("Single Sign-On identifier")),
		mcp.WithBoolean("extern", mcp.Description("Mark the user as an external collaborator")),
		mcp.WithBoolean("app_access", mcp.Description("Right: access to the application")),
		mcp.WithBoolean("administrator", mcp.Description("Right: administrator")),
		mcp.WithBoolean("manager", mcp.Description("Right: manager")),
		mcp.WithBoolean("pmo", mcp.Description("Right: PMO")),
		mcp.WithBoolean("limited_access", mcp.Description("Right: limited access")),
		mcp.WithBoolean("create_project", mcp.Description("Right: create projects")),
		mcp.WithBoolean("manage_self", mcp.Description("Right: manage own assignments")),
	}
}

func userWrite(req mcp.CallToolRequest) tp.UserWrite {
	f := fields(req)
	w := tp.UserWrite{
		Firstname:         f.str("firstname"),
		Lastname:          f.str("lastname"),
		Email:             f.str("email"),
		ExternalID:        f.str("external_id"),
		AvailabilityStart: f.str("availability_start"),
		AvailabilityEnd:   f.str("availability_end"),
		OBS:               f.num("obs"),
		Profile:           f.num("profile"),
		Gender:            f.str("gender"),
		Locale:            f.str("locale"),
		SSOUID:            f.str("sso_uid"),
		Extern:            f.boolean("extern"),
	}

	rights := tp.UserRightsWrite{
		AppAccess:     f.boolean("app_access"),
		Administrator: f.boolean("administrator"),
		Manager:       f.boolean("manager"),
		PMO:           f.boolean("pmo"),
		LimitedAccess: f.boolean("limited_access"),
		CreateProject: f.boolean("create_project"),
		ManageSelf:    f.boolean("manage_self"),
	}
	if !isEmpty(rights.AppAccess, rights.Administrator, rights.Manager, rights.PMO,
		rights.LimitedAccess, rights.CreateProject, rights.ManageSelf) {
		w.Rights = &rights
	}
	return w
}

func userSummary(req mcp.CallToolRequest, w tp.UserWrite) []string {
	summary := []string{
		summarize("First name", req.GetString("firstname", "")),
		summarize("Last name", req.GetString("lastname", "")),
		summarize("Email", req.GetString("email", "")),
		summarize("Availability", strings.TrimSpace(req.GetString("availability_start", "")+" → "+req.GetString("availability_end", ""))),
		intSummary("Organizational unit", w.OBS, "#%d"),
		intSummary("Skill profile", w.Profile, "#%d"),
	}
	if w.Extern != nil {
		summary = append(summary, fmt.Sprintf("External collaborator: %t", *w.Extern))
	}
	if r := w.Rights; r != nil {
		var changed []string
		for label, v := range map[string]*bool{
			"app access": r.AppAccess, "administrator": r.Administrator, "manager": r.Manager,
			"pmo": r.PMO, "limited access": r.LimitedAccess, "create project": r.CreateProject,
			"manage self": r.ManageSelf,
		} {
			if v != nil {
				changed = append(changed, fmt.Sprintf("%s=%t", label, *v))
			}
		}
		sort.Strings(changed)
		summary = append(summary, "Rights: "+strings.Join(changed, ", "))
	}
	return summary
}

func registerCreateUser(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithString("firstname", mcp.Description("First name"), mcp.Required()),
		mcp.WithString("lastname", mcp.Description("Last name"), mcp.Required()),
		mcp.WithString("external_id", mcp.Description("Id in another system (UI: external reference)")),
	}, userWriteOptions()...)

	tool := newWriteTool("create_user",
		"Create a TimePerformance user account. Needs api:write (back-office credentials): user credentials are read-only.",
		false, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}
		payload := userWrite(req)
		if payload.Firstname == nil || payload.Lastname == nil {
			return errf("firstname and lastname are required"), nil
		}

		if abort := confirmWrite(ctx, s, "Create a TimePerformance user account", why, userSummary(req, payload)); abort != nil {
			return abort, nil
		}

		user, err := client.CreateUser(payload)
		if err != nil {
			return errf("create user failed: %v", err), nil
		}
		return mcp.NewToolResultText("User created.\n\n" + FormatUser(user)), nil
	})
}

func registerUpdateUser(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithString("user",
			mcp.Description("User numeric id, full name, email, or \"me\""),
			mcp.Required(),
		),
		mcp.WithString("firstname", mcp.Description("New first name")),
		mcp.WithString("lastname", mcp.Description("New last name")),
		mcp.WithString("external_id", mcp.Description("New external reference")),
	}, userWriteOptions()...)

	tool := newWriteTool("update_user",
		"Update a user account, including its application rights. Only the fields you pass are changed. Needs api:write (back-office credentials).",
		true, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveUser(client, req, "user")
		if abort != nil {
			return abort, nil
		}
		why, abortSummary := requireChangeSummary(req)
		if abortSummary != nil {
			return abortSummary, nil
		}
		payload := userWrite(req)
		if isEmpty(payload.Firstname, payload.Lastname, payload.Email, payload.ExternalID,
			payload.AvailabilityStart, payload.AvailabilityEnd, payload.OBS, payload.Profile,
			payload.Gender, payload.Locale, payload.SSOUID, payload.Extern) && payload.Rights == nil {
			return errf("nothing to update: pass at least one field to change"), nil
		}

		summary := append([]string{fmt.Sprintf("User: #%d", id)}, userSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Update a TimePerformance user account", why, summary); abort != nil {
			return abort, nil
		}

		user, err := client.UpdateUser(id, payload)
		if err != nil {
			return errf("update user failed: %v", err), nil
		}
		return mcp.NewToolResultText("User updated.\n\n" + FormatUser(user)), nil
	})
}

func registerArchiveUser(s *server.MCPServer, client *tp.Client) {
	tool := newWriteTool("archive_user",
		"Archive a user account: the person keeps their history but loses access and no longer appears in the active directory. Needs api:write (back-office credentials).",
		true,
		mcp.WithString("user",
			mcp.Description("User numeric id, full name or email"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, abort := resolveUser(client, req, "user")
		if abort != nil {
			return abort, nil
		}
		why, abortSummary := requireChangeSummary(req)
		if abortSummary != nil {
			return abortSummary, nil
		}

		label := fmt.Sprintf("User: #%d", id)
		if user, err := client.GetUser(id); err == nil {
			label = fmt.Sprintf("User: %s (#%d, %s)", user.Name, user.ID, user.Email)
		}
		if abort := confirmWrite(ctx, s, "Archive a TimePerformance user account", why, []string{label}); abort != nil {
			return abort, nil
		}

		if err := client.ArchiveUser(id); err != nil {
			return errf("archive user failed: %v", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("User #%d archived.", id)), nil
	})
}
