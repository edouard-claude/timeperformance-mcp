package tools

import (
	"fmt"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/mcp"
)

// newReadTool builds a read-only tool with the standard annotations.
func newReadTool(name, description string, opts ...mcp.ToolOption) mcp.Tool {
	base := []mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	}
	return mcp.NewTool(name, append(base, opts...)...)
}

// newWriteTool builds a write tool: standard annotations plus the mandatory
// change_summary parameter consumed by the confirmation flow.
func newWriteTool(name, description string, destructive bool, opts ...mcp.ToolOption) mcp.Tool {
	base := []mcp.ToolOption{
		mcp.WithDescription(description),
		withChangeSummary(),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(destructive),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	}
	return mcp.NewTool(name, append(base, opts...)...)
}

func errf(format string, a ...any) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf(format, a...))
}

// args wraps the raw tool arguments so optional fields can be told apart from
// fields left out: a PATCH must only carry what the caller actually set.
type args map[string]any

func fields(req mcp.CallToolRequest) args { return args(req.GetArguments()) }

func (a args) str(key string) *string {
	v, ok := a[key]
	if !ok || v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		s = fmt.Sprintf("%v", v)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func (a args) num(key string) *int {
	f := a.float(key)
	if f == nil {
		return nil
	}
	n := int(*f)
	return &n
}

func (a args) float(key string) *float64 {
	v, ok := a[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case float64:
		return &t
	case int:
		f := float64(t)
		return &f
	case string:
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%g", &f); err == nil {
			return &f
		}
	}
	return nil
}

func (a args) boolean(key string) *bool {
	v, ok := a[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case bool:
		return &t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "1", "on":
			b := true
			return &b
		case "false", "no", "0", "off":
			b := false
			return &b
		}
	}
	return nil
}

// isEmpty reports whether a write payload would carry no field at all.
func isEmpty(pairs ...any) bool {
	for _, p := range pairs {
		switch v := p.(type) {
		case nil:
		case *string:
			if v != nil {
				return false
			}
		case *int:
			if v != nil {
				return false
			}
		case *bool:
			if v != nil {
				return false
			}
		case *tp.UnitValue:
			if v != nil {
				return false
			}
		case []int:
			if len(v) > 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// resolveProject turns the "project" argument (id or name) into a project id.
func resolveProject(client *tp.Client, req mcp.CallToolRequest) (int, *mcp.CallToolResult) {
	ref := strings.TrimSpace(req.GetString("project", ""))
	if ref == "" {
		return 0, errf("project is required (numeric id or exact project name)")
	}
	id, err := client.ResolveProjectID(ref)
	if err != nil {
		return 0, errf("%v", err)
	}
	return id, nil
}

// resolveUser turns the "user" argument (id, name, email or "me") into a user id.
func resolveUser(client *tp.Client, req mcp.CallToolRequest, key string) (int, *mcp.CallToolResult) {
	ref := strings.TrimSpace(req.GetString(key, ""))
	if ref == "" {
		return 0, errf("%s is required (numeric id, full name, email, or \"me\")", key)
	}
	id, err := client.ResolveUserID(ref)
	if err != nil {
		return 0, errf("%v", err)
	}
	return id, nil
}

// requireDates checks the mandatory period of the time endpoints.
func requireDates(req mcp.CallToolRequest) (string, string, *mcp.CallToolResult) {
	first := strings.TrimSpace(req.GetString("first_day", ""))
	last := strings.TrimSpace(req.GetString("last_day", ""))
	if first == "" || last == "" {
		return "", "", errf("first_day and last_day are required (ISO 8601 dates, e.g. 2026-09-01)")
	}
	return first, last, nil
}
