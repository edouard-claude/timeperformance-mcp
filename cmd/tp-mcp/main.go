package main

import (
	"fmt"
	"os"

	"github.com/edouard-claude/timeperformance-mcp/internal/cli"
	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/edouard-claude/timeperformance-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

var version = "1.0.0"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches between MCP stdio mode (default, or explicit "mcp"/"serve")
// and CLI mode (any other subcommand). Returns the process exit code.
func run(args []string) int {
	// Let --help work without credentials configured.
	if len(args) > 0 {
		switch args[0] {
		case "help", "--help", "-h":
			return cli.Run(args, nil)
		case "version", "--version":
			fmt.Println("tp-mcp " + version)
			return 0
		}
	}

	client, err := tp.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "timeperformance client: %v\n", err)
		return 1
	}

	if len(args) == 0 || args[0] == "mcp" || args[0] == "serve" {
		return runMCP(client)
	}
	return cli.Run(args, client)
}

func runMCP(client *tp.Client) int {
	s := server.NewMCPServer(
		"timeperformance-mcp",
		version,
		server.WithElicitation(),
		server.WithInstructions(`TimePerformance access via its REST API v4 (projects, tasks, deliverables, phases, expenses, teams, users, timesheets and portfolios).

Resolve a project or a user by name: the "project" and "user" parameters accept a numeric id, an exact project name, or for users a full name, an email, or "me". Reference ids expected by the write tools (skill profiles, organizational units, project states) come from list_reference_data.

Reports (get_project_report, get_portfolio_progress_report) return the API's raw JSON: read the indicators you need from it rather than assuming a shape.

Write tools ask the user to confirm through an elicitation prompt before anything reaches TimePerformance. An aborted write means the user said no: report it and do not retry without new instructions. Writes need back-office API credentials — user credentials are read-only and any write with them fails with HTTP 403.`),
	)

	tools.RegisterAll(s, client)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
