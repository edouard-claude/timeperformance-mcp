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

func registerListPortfolios(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("list_portfolios",
		"List the portfolios (sets of projects) with the projects they contain.",
		mcp.WithBoolean("include_archived",
			mcp.Description("Include archived portfolios (default: false)"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		portfolios, err := client.ListPortfolios(req.GetBool("include_archived", false))
		if err != nil {
			return errf("list portfolios failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatPortfolios(portfolios)), nil
	})
}

func registerPortfolioReport(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_portfolio_progress_report",
		"Get the progress report of a portfolio: per-project progress, cost, effort and RAG indicators. Returned as raw JSON.",
		mcp.WithString("portfolio",
			mcp.Description("Portfolio numeric id or exact name"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ref := strings.TrimSpace(req.GetString("portfolio", ""))
		if ref == "" {
			return errf("portfolio is required (numeric id or exact name)"), nil
		}
		id, err := client.ResolvePortfolioID(ref)
		if err != nil {
			return errf("%v", err), nil
		}
		raw, err := client.PortfolioProgressReport(id)
		if err != nil {
			return errf("get portfolio progress report failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatJSON(fmt.Sprintf("Portfolio #%d — progress report", id), raw)), nil
	})
}

func registerListReference(s *server.MCPServer, client *tp.Client) {
	kinds := make([]string, 0, len(tp.ReferencePath))
	for k := range tp.ReferencePath {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	tool := newReadTool("list_reference_data",
		"List one of the configuration reference lists, whose ids the write tools expect: obs (organizational units), profiles (skill profiles), npactivities (non-project activities and unavailabilities), projectstates (project workflow states), customforms (datasheet templates).",
		mcp.WithString("kind",
			mcp.Description("Reference list: "+strings.Join(kinds, ", ")),
			mcp.Required(),
			mcp.Enum(kinds...),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kind := strings.ToLower(strings.TrimSpace(req.GetString("kind", "")))
		if _, ok := tp.ReferencePath[kind]; !ok {
			return errf("unknown kind %q — pick one of: %s", kind, strings.Join(kinds, ", ")), nil
		}
		items, err := client.ListReference(kind)
		if err != nil {
			return errf("list %s failed: %v", kind, err), nil
		}
		return mcp.NewToolResultText(FormatReference(kind, items)), nil
	})
}
