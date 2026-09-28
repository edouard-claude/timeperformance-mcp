package tools

import (
	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers every MCP tool on the server.
func RegisterAll(s *server.MCPServer, client *tp.Client) {
	// Reads
	registerWhoAmI(s, client)
	registerListProjects(s, client)
	registerGetProject(s, client)
	registerProjectReport(s, client)
	registerListProjectTeam(s, client)
	registerListProjectTasks(s, client)
	registerGetTask(s, client)
	registerListDeliverables(s, client)
	registerGetDeliverable(s, client)
	registerListPhases(s, client)
	registerGetPhase(s, client)
	registerListExpenses(s, client)
	registerGetExpense(s, client)
	registerListUsers(s, client)
	registerGetUser(s, client)
	registerUserTimesheet(s, client)
	registerUserTimeReport(s, client)
	registerListUserTasks(s, client)
	registerUserAssignments(s, client)
	registerListPortfolios(s, client)
	registerPortfolioReport(s, client)
	registerListReference(s, client)

	// Writes (each one goes through the confirmation flow)
	registerCreateProject(s, client)
	registerUpdateProject(s, client)
	registerCreateTask(s, client)
	registerUpdateTask(s, client)
	registerCreateDeliverable(s, client)
	registerUpdateDeliverable(s, client)
	registerCreatePhase(s, client)
	registerUpdatePhase(s, client)
	registerCreateExpense(s, client)
	registerUpdateExpense(s, client)
	registerCreateUser(s, client)
	registerUpdateUser(s, client)
	registerArchiveUser(s, client)
	registerManageProjectMember(s, client)
	registerSetUserLeave(s, client)
	registerDeleteElement(s, client)
}
