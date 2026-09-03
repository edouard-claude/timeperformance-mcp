package tools

import (
	"context"
	"fmt"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerListExpenses(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("list_project_expenses",
		"List the expenses recorded on a project, with amount, date, counterparty and the deliverable they are attached to.",
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
		expenses, err := client.ListExpenses(id)
		if err != nil {
			return errf("list expenses failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatExpenses(expenses, fmt.Sprintf("Expenses of project #%d", id))), nil
	})
}

func registerGetExpense(s *server.MCPServer, client *tp.Client) {
	tool := newReadTool("get_expense",
		"Get one expense by id: amount, date, counterparty, settlement and linked deliverable.",
		mcp.WithNumber("expense_id",
			mcp.Description("Expense numeric id"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetInt("expense_id", 0)
		if id == 0 {
			return errf("expense_id is required"), nil
		}
		expense, err := client.GetExpense(id)
		if err != nil {
			return errf("get expense failed: %v", err), nil
		}
		return mcp.NewToolResultText(FormatExpense(expense)), nil
	})
}

// expenseWriteOptions are the writable fields of an expense.
func expenseWriteOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("description", mcp.Description("Description")),
		mcp.WithNumber("amount", mcp.Description("Monetary amount")),
		mcp.WithString("currency", mcp.Description("Currency code of the amount (e.g. EUR). Required when amount is set")),
		mcp.WithString("date", mcp.Description("Expense date (ISO 8601 date, e.g. 2026-09-15)")),
		mcp.WithString("counterparty", mcp.Description("Supplier or counterparty name")),
		mcp.WithBoolean("smoothed", mcp.Description("Smooth the expense over time for cost accounting")),
		mcp.WithNumber("deliverable", mcp.Description("Deliverable id the expense is attached to")),
		mcp.WithString("external_id", mcp.Description("Id in another system (UI: external reference)")),
	}
}

func expenseWrite(req mcp.CallToolRequest) (tp.ExpenseWrite, *mcp.CallToolResult) {
	f := fields(req)
	w := tp.ExpenseWrite{
		Name:         f.str("name"),
		Description:  f.str("description"),
		Date:         f.str("date"),
		Counterparty: f.str("counterparty"),
		Smoothed:     f.boolean("smoothed"),
		Deliverable:  f.num("deliverable"),
		ExternalID:   f.str("external_id"),
	}
	if amount := f.float("amount"); amount != nil {
		currency := strings.TrimSpace(req.GetString("currency", ""))
		if currency == "" {
			return w, errf("currency is required when amount is set (e.g. EUR)")
		}
		w.Amount = &tp.UnitValue{Val: *amount, Unit: currency}
	}
	return w, nil
}

func expenseSummary(req mcp.CallToolRequest, w tp.ExpenseWrite) []string {
	summary := []string{
		summarize("Label", req.GetString("name", "")),
		summarize("Description", req.GetString("description", "")),
		summarize("Date", req.GetString("date", "")),
		summarize("Counterparty", req.GetString("counterparty", "")),
		intSummary("Deliverable", w.Deliverable, "#%d"),
	}
	if w.Amount != nil {
		summary = append(summary, "Amount: "+amountLabel(w.Amount))
	}
	if w.Smoothed != nil {
		summary = append(summary, fmt.Sprintf("Smoothed: %t", *w.Smoothed))
	}
	return summary
}

func registerCreateExpense(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithString("project", mcp.Description("Project numeric id or exact name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Expense label"), mcp.Required()),
	}, expenseWriteOptions()...)

	tool := newWriteTool("create_expense",
		"Record an expense on a project. Needs api:write (back-office credentials): user credentials are read-only.",
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
		payload, abortPayload := expenseWrite(req)
		if abortPayload != nil {
			return abortPayload, nil
		}
		if payload.Name == nil {
			return errf("name is required"), nil
		}

		summary := append([]string{fmt.Sprintf("Project: #%d", projectID)}, expenseSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Record a TimePerformance expense", why, summary); abort != nil {
			return abort, nil
		}

		expense, err := client.CreateExpense(projectID, payload)
		if err != nil {
			return errf("create expense failed: %v", err), nil
		}
		return mcp.NewToolResultText("Expense created.\n\n" + FormatExpense(expense)), nil
	})
}

func registerUpdateExpense(s *server.MCPServer, client *tp.Client) {
	opts := append([]mcp.ToolOption{
		mcp.WithNumber("expense_id", mcp.Description("Expense numeric id"), mcp.Required()),
		mcp.WithString("name", mcp.Description("New label")),
	}, expenseWriteOptions()...)

	tool := newWriteTool("update_expense",
		"Update an expense. Only the fields you pass are changed. Needs api:write (back-office credentials).",
		true, opts...)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetInt("expense_id", 0)
		if id == 0 {
			return errf("expense_id is required"), nil
		}
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}
		payload, abortPayload := expenseWrite(req)
		if abortPayload != nil {
			return abortPayload, nil
		}
		if isEmpty(payload.Name, payload.Description, payload.Date, payload.Counterparty, payload.Smoothed, payload.Deliverable, payload.ExternalID, payload.Amount) {
			return errf("nothing to update: pass at least one field to change"), nil
		}

		summary := append([]string{fmt.Sprintf("Expense: #%d", id)}, expenseSummary(req, payload)...)
		if abort := confirmWrite(ctx, s, "Update a TimePerformance expense", why, summary); abort != nil {
			return abort, nil
		}

		expense, err := client.UpdateExpense(id, payload)
		if err != nil {
			return errf("update expense failed: %v", err), nil
		}
		return mcp.NewToolResultText("Expense updated.\n\n" + FormatExpense(expense)), nil
	})
}
