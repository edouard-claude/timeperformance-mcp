// Package cli exposes the TimePerformance operations as subcommands of the
// binary, so the same executable can run either as an MCP server (stdio
// JSON-RPC) or as a plain CLI tool composable from a shell.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/edouard-claude/timeperformance-mcp/internal/tools"
)

// Run dispatches a CLI subcommand. Returns a process exit code.
func Run(args []string, client *tp.Client) int {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return 0

	// reads
	case "whoami":
		return cmdWhoAmI(client, args[1:])
	case "list-projects":
		return cmdListProjects(client, args[1:])
	case "get-project":
		return cmdGetProject(client, args[1:])
	case "project-report":
		return cmdProjectReport(client, args[1:])
	case "list-team":
		return cmdListTeam(client, args[1:])
	case "list-tasks":
		return cmdListTasks(client, args[1:])
	case "get-task":
		return cmdGetTask(client, args[1:])
	case "list-deliverables":
		return cmdListWBS(client, args[1:], "deliverables")
	case "get-deliverable":
		return cmdGetWBS(client, args[1:], "deliverable")
	case "list-phases":
		return cmdListWBS(client, args[1:], "phases")
	case "get-phase":
		return cmdGetWBS(client, args[1:], "phase")
	case "list-expenses":
		return cmdListExpenses(client, args[1:])
	case "get-expense":
		return cmdGetExpense(client, args[1:])
	case "list-users":
		return cmdListUsers(client, args[1:])
	case "get-user":
		return cmdGetUser(client, args[1:])
	case "timesheet":
		return cmdTimesheet(client, args[1:])
	case "time-report":
		return cmdTimeReport(client, args[1:])
	case "user-tasks":
		return cmdUserTasks(client, args[1:])
	case "assignments":
		return cmdAssignments(client, args[1:])
	case "list-portfolios":
		return cmdListPortfolios(client, args[1:])
	case "portfolio-report":
		return cmdPortfolioReport(client, args[1:])
	case "reference":
		return cmdReference(client, args[1:])

	// writes
	case "create-project":
		return cmdCreateProject(client, args[1:])
	case "update-project":
		return cmdUpdateProject(client, args[1:])
	case "create-task":
		return cmdCreateTask(client, args[1:])
	case "update-task":
		return cmdUpdateTask(client, args[1:])
	case "create-deliverable":
		return cmdCreateWBS(client, args[1:], true)
	case "update-deliverable":
		return cmdUpdateWBS(client, args[1:], true)
	case "create-phase":
		return cmdCreateWBS(client, args[1:], false)
	case "update-phase":
		return cmdUpdateWBS(client, args[1:], false)
	case "create-expense":
		return cmdCreateExpense(client, args[1:])
	case "update-expense":
		return cmdUpdateExpense(client, args[1:])
	case "create-user":
		return cmdCreateUser(client, args[1:])
	case "update-user":
		return cmdUpdateUser(client, args[1:])
	case "archive-user":
		return cmdArchiveUser(client, args[1:])
	case "team-member":
		return cmdTeamMember(client, args[1:])
	case "leave":
		return cmdLeave(client, args[1:])
	case "delete":
		return cmdDelete(client, args[1:])

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: tp-mcp <command> [options]

Reads:
  whoami                       Identity behind the configured credentials
  list-projects                Projects (--name, --archived, --templates)
  get-project <project>        One project by id or name
  project-report <project>     Report (--kind progress|roadmap|brief|baseline|
                               config|workload|actuals|assignments|datasheets|
                               lastmodified|risks)
  list-team <project>          Project team members
  list-tasks <project>         Project tasks (--not-closed, --from, --to)
  get-task <id>                One task
  list-deliverables <project>  Deliverables tree
  get-deliverable <id>         One deliverable
  list-phases <project>        Phases tree
  get-phase <id>               One phase
  list-expenses <project>      Project expenses
  get-expense <id>             One expense
  list-users                   User directory (--query, --archived)
  get-user <user>              One user (id, name, email or "me")
  timesheet <user>             Per-task timesheet (--from, --to required)
  time-report <user>           Per-activity report (--from, --to required)
  user-tasks <user>            Tasks of a user (--scope todo, --project)
  assignments <user>           Half-day schedule (--from, --to required)
  list-portfolios              Portfolios (--archived)
  portfolio-report <portfolio> Portfolio progress report
  reference --kind K           obs|profiles|npactivities|projectstates|customforms

Writes (need back-office credentials, and --yes to actually send):
  create-project               --name, --project-manager, …
  update-project <project>     --name, --sponsor, --state, …
  create-task                  --project, --name, --performer, …
  update-task <id>             --name, --performer, --due-date, …
  create-deliverable           --project or --parent, --name, …
  update-deliverable <id>      --name, --state, --first-day, …
  create-phase                 --project or --parent, --name, …
  update-phase <id>            --name, --state, --first-day, …
  create-expense               --project, --name, --amount, --currency, …
  update-expense <id>          --amount, --currency, --date, …
  create-user                  --firstname, --lastname, --email, …
  update-user <user>           --email, --profile, rights flags, …
  archive-user <user>          Archive an account
  team-member <project>        --user, --action add|set_rights|remove
  leave <user>                 --type, --from, --to, --remove, --mode, …
  delete                       --kind task|deliverable|phase|expense|datasheet --id

Server:
  mcp                          Run as MCP server over stdio (also the default
                               when invoked with no arguments)

Run 'tp-mcp <command> --help' for command-specific options.

Environment:
  TIMEPERFORMANCE_URL          Base URL (default: https://pma.timeperformance.com)
  TIMEPERFORMANCE_LOGIN        API credential id      (required)
  TIMEPERFORMANCE_PASSWORD     API credential secret  (required)
  TIMEPERFORMANCE_AUTO_WRITE   1 to skip the MCP confirmation prompt
`)
}

// --- helpers ---

func failf(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 1
}

func out(text string) int {
	fmt.Println(strings.TrimRight(text, "\n"))
	return 0
}

// flags wraps a FlagSet and remembers which flags were actually passed, so a
// PATCH only carries the fields the caller set.
type flags struct {
	fs     *flag.FlagSet
	strs   map[string]*string
	ints   map[string]*int
	floats map[string]*float64
	bools  map[string]*bool
	seen   map[string]bool
}

func newFlags(name string) *flags {
	return &flags{
		fs:     flag.NewFlagSet(name, flag.ContinueOnError),
		strs:   map[string]*string{},
		ints:   map[string]*int{},
		floats: map[string]*float64{},
		bools:  map[string]*bool{},
		seen:   map[string]bool{},
	}
}

func (f *flags) str(name, usage string) {
	f.strs[name] = f.fs.String(name, "", usage)
}

func (f *flags) int(name, usage string) {
	f.ints[name] = f.fs.Int(name, 0, usage)
}

func (f *flags) float(name, usage string) {
	f.floats[name] = f.fs.Float64(name, 0, usage)
}

func (f *flags) bool(name, usage string) {
	f.bools[name] = f.fs.Bool(name, false, usage)
}

// reorder moves positional arguments after the flags, so `timesheet me --from X`
// works as well as `timesheet --from X me`: the stdlib flag package stops
// parsing at the first non-flag argument.
func (f *flags) reorder(args []string) []string {
	var flagArgs, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(arg) < 2 || !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		flagArgs = append(flagArgs, arg)
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			continue
		}
		// A non-boolean flag takes the next argument as its value.
		if fl := f.fs.Lookup(name); fl != nil && !isBoolFlag(fl) && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return append(flagArgs, positional...)
}

func isBoolFlag(fl *flag.Flag) bool {
	b, ok := fl.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// parse parses the arguments and records which flags were provided. positional
// documents the expected trailing arguments in the usage line.
func (f *flags) parse(args []string, positional string) (rest []string, code int, ok bool) {
	f.fs.Usage = func() {
		w := f.fs.Output()
		fmt.Fprintf(w, "Usage: tp-mcp %s [flags] %s\n", f.fs.Name(), positional)
		f.fs.PrintDefaults()
	}
	if err := f.fs.Parse(f.reorder(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, 0, false
		}
		return nil, 2, false
	}
	f.fs.Visit(func(fl *flag.Flag) { f.seen[fl.Name] = true })
	return f.fs.Args(), 0, true
}

func (f *flags) optStr(name string) *string {
	if !f.seen[name] {
		return nil
	}
	v := strings.TrimSpace(*f.strs[name])
	return &v
}

func (f *flags) optInt(name string) *int {
	if !f.seen[name] {
		return nil
	}
	return f.ints[name]
}

func (f *flags) optFloat(name string) *float64 {
	if !f.seen[name] {
		return nil
	}
	return f.floats[name]
}

func (f *flags) optBool(name string) *bool {
	if !f.seen[name] {
		return nil
	}
	return f.bools[name]
}

func (f *flags) getStr(name string) string { return strings.TrimSpace(*f.strs[name]) }
func (f *flags) getInt(name string) int    { return *f.ints[name] }
func (f *flags) getBool(name string) bool  { return *f.bools[name] }

// firstArg returns the required positional argument.
func firstArg(rest []string, what string) (string, int, bool) {
	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "missing positional argument: %s\n", what)
		return "", 2, false
	}
	if len(rest) > 1 {
		fmt.Fprintf(os.Stderr, "unexpected extra arguments: %s\n", strings.Join(rest[1:], " "))
		return "", 2, false
	}
	return rest[0], 0, true
}

func firstIntArg(rest []string, what string) (int, int, bool) {
	raw, code, ok := firstArg(rest, what)
	if !ok {
		return 0, code, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s must be a number, got %q\n", what, raw)
		return 0, 2, false
	}
	return id, 0, true
}

// confirmed gates every write: without --yes the command prints what it would
// send and changes nothing. This mirrors the MCP elicitation prompt.
func confirmed(f *flags, title string, summary []string) bool {
	if f.getBool("yes") {
		return true
	}
	fmt.Printf("%s\n\n", title)
	for _, line := range summary {
		if strings.TrimSpace(line) != "" {
			fmt.Println(line)
		}
	}
	fmt.Println("\nNothing was sent. Re-run with --yes to apply.")
	return false
}

// --- read commands ---

func cmdWhoAmI(client *tp.Client, args []string) int {
	f := newFlags("whoami")
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}
	who, err := client.WhoAmI()
	if err != nil {
		return failf("whoami failed: %v", err)
	}
	return out(fmt.Sprintf("Authenticated as: %s\nInstance: %s", who, client.BaseURL()))
}

func cmdListProjects(client *tp.Client, args []string) int {
	f := newFlags("list-projects")
	f.str("name", "Filter on project or client name (case-insensitive)")
	f.bool("archived", "Include archived projects")
	f.bool("templates", "List project templates instead of projects")
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}

	var (
		projects []tp.Project
		err      error
		title    = "Projects"
	)
	if f.getBool("templates") {
		title = "Project templates"
		projects, err = client.ListProjectTemplates(f.getBool("archived"))
	} else {
		projects, err = client.ListProjects(f.getBool("archived"))
	}
	if err != nil {
		return failf("list projects failed: %v", err)
	}

	if filter := strings.ToLower(f.getStr("name")); filter != "" {
		kept := projects[:0:0]
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.Name), filter) || strings.Contains(strings.ToLower(p.Sponsor), filter) {
				kept = append(kept, p)
			}
		}
		projects = kept
	}
	return out(tools.FormatProjects(projects, title))
}

func cmdGetProject(client *tp.Client, args []string) int {
	f := newFlags("get-project")
	rest, code, ok := f.parse(args, "<project id or name>")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "project")
	if !ok {
		return code
	}
	id, err := client.ResolveProjectID(ref)
	if err != nil {
		return failf("%v", err)
	}
	project, err := client.GetProject(id)
	if err != nil {
		return failf("get project failed: %v", err)
	}
	return out(tools.FormatProject(project))
}

func cmdProjectReport(client *tp.Client, args []string) int {
	f := newFlags("project-report")
	f.str("kind", "Report: progress, roadmap, brief, baseline, config, workload, actuals, assignments, datasheets, lastmodified, risks")
	f.bool("with-deliverables", "progress only: include the deliverables breakdown")
	f.bool("with-tasks", "roadmap only: include tasks")
	rest, code, ok := f.parse(args, "<project id or name>")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "project")
	if !ok {
		return code
	}
	kind := strings.ToLower(f.getStr("kind"))
	if kind == "" {
		return failf("--kind is required")
	}
	id, err := client.ResolveProjectID(ref)
	if err != nil {
		return failf("%v", err)
	}

	if kind == "risks" {
		risks, err := client.ListProjectRisks(id)
		if err != nil {
			return failf("list risks failed: %v", err)
		}
		return out(tools.FormatRisks(risks, id))
	}

	params := map[string]string{}
	if kind == "progress" && f.getBool("with-deliverables") {
		params["withDeliverables"] = "true"
	}
	if kind == "roadmap" && f.getBool("with-tasks") {
		params["tasks"] = "true"
	}
	values := toValues(params)
	raw, err := client.ProjectReport(id, kind, values)
	if err != nil {
		return failf("get %s report failed: %v", kind, err)
	}
	return out(tools.FormatJSON(fmt.Sprintf("Project #%d — %s report", id, kind), raw))
}

func cmdListTeam(client *tp.Client, args []string) int {
	f := newFlags("list-team")
	rest, code, ok := f.parse(args, "<project id or name>")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "project")
	if !ok {
		return code
	}
	id, err := client.ResolveProjectID(ref)
	if err != nil {
		return failf("%v", err)
	}
	members, err := client.ListTeam(id)
	if err != nil {
		return failf("list team failed: %v", err)
	}
	return out(tools.FormatTeam(members, id))
}

func cmdListTasks(client *tp.Client, args []string) int {
	f := newFlags("list-tasks")
	f.bool("not-closed", "Only tasks that are neither completed nor cancelled")
	f.str("from", "Period start (ISO 8601 date)")
	f.str("to", "Period end (ISO 8601 date)")
	rest, code, ok := f.parse(args, "<project id or name>")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "project")
	if !ok {
		return code
	}
	id, err := client.ResolveProjectID(ref)
	if err != nil {
		return failf("%v", err)
	}
	tasks, err := client.ListProjectTasks(id, tp.TaskFilter{
		NotClosed:   f.getBool("not-closed"),
		PeriodStart: f.getStr("from"),
		PeriodEnd:   f.getStr("to"),
	})
	if err != nil {
		return failf("list tasks failed: %v", err)
	}
	return out(tools.FormatTasks(tasks, fmt.Sprintf("Tasks of project #%d", id)))
}

func cmdGetTask(client *tp.Client, args []string) int {
	f := newFlags("get-task")
	rest, code, ok := f.parse(args, "<task id>")
	if !ok {
		return code
	}
	id, code, ok := firstIntArg(rest, "task id")
	if !ok {
		return code
	}
	task, err := client.GetTask(id)
	if err != nil {
		return failf("get task failed: %v", err)
	}
	return out(tools.FormatTask(task))
}

func cmdListWBS(client *tp.Client, args []string, kind string) int {
	f := newFlags("list-" + kind)
	rest, code, ok := f.parse(args, "<project id or name>")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "project")
	if !ok {
		return code
	}
	id, err := client.ResolveProjectID(ref)
	if err != nil {
		return failf("%v", err)
	}

	var items []tp.WBSElement
	if kind == "deliverables" {
		items, err = client.ListDeliverables(id)
	} else {
		items, err = client.ListPhases(id)
	}
	if err != nil {
		return failf("list %s failed: %v", kind, err)
	}
	return out(tools.FormatWBSTree(items, fmt.Sprintf("%s of project #%d", titleCase(kind), id)))
}

func cmdGetWBS(client *tp.Client, args []string, kind string) int {
	f := newFlags("get-" + kind)
	rest, code, ok := f.parse(args, "<"+kind+" id>")
	if !ok {
		return code
	}
	id, code, ok := firstIntArg(rest, kind+" id")
	if !ok {
		return code
	}

	var (
		item *tp.WBSElement
		err  error
	)
	if kind == "deliverable" {
		item, err = client.GetDeliverable(id)
	} else {
		item, err = client.GetPhase(id)
	}
	if err != nil {
		return failf("get %s failed: %v", kind, err)
	}
	return out(tools.FormatWBSElement(item, titleCase(kind)))
}

func cmdListExpenses(client *tp.Client, args []string) int {
	f := newFlags("list-expenses")
	rest, code, ok := f.parse(args, "<project id or name>")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "project")
	if !ok {
		return code
	}
	id, err := client.ResolveProjectID(ref)
	if err != nil {
		return failf("%v", err)
	}
	expenses, err := client.ListExpenses(id)
	if err != nil {
		return failf("list expenses failed: %v", err)
	}
	return out(tools.FormatExpenses(expenses, fmt.Sprintf("Expenses of project #%d", id)))
}

func cmdGetExpense(client *tp.Client, args []string) int {
	f := newFlags("get-expense")
	rest, code, ok := f.parse(args, "<expense id>")
	if !ok {
		return code
	}
	id, code, ok := firstIntArg(rest, "expense id")
	if !ok {
		return code
	}
	expense, err := client.GetExpense(id)
	if err != nil {
		return failf("get expense failed: %v", err)
	}
	return out(tools.FormatExpense(expense))
}

func cmdListUsers(client *tp.Client, args []string) int {
	f := newFlags("list-users")
	f.str("query", "Filter on name or email (case-insensitive)")
	f.bool("archived", "Include archived accounts")
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}
	users, err := client.ListUsers()
	if err != nil {
		return failf("list users failed: %v", err)
	}
	query := strings.ToLower(f.getStr("query"))
	kept := users[:0:0]
	for _, u := range users {
		if u.Archived && !f.getBool("archived") {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(u.Name), query) && !strings.Contains(strings.ToLower(u.Email), query) {
			continue
		}
		kept = append(kept, u)
	}
	return out(tools.FormatUsers(kept, "Users"))
}

func cmdGetUser(client *tp.Client, args []string) int {
	f := newFlags("get-user")
	rest, code, ok := f.parse(args, "<user id, name, email or \"me\">")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "user")
	if !ok {
		return code
	}
	id, err := client.ResolveUserID(ref)
	if err != nil {
		return failf("%v", err)
	}
	user, err := client.GetUser(id)
	if err != nil {
		return failf("get user failed: %v", err)
	}
	return out(tools.FormatUser(user))
}

// timeFlags declares the period options shared by the time commands.
func timeFlags(f *flags) {
	f.str("from", "First day of the period (ISO 8601 date, required)")
	f.str("to", "Last day of the period (ISO 8601 date, required)")
	f.float("hours-per-day", "Equalize the report to this many hours per worked day")
	f.float("half-day-threshold", "With --hours-per-day: hours below which a day counts as a half-day")
}

func timeParams(f *flags) (tp.TimeParams, bool) {
	first, last := f.getStr("from"), f.getStr("to")
	if first == "" || last == "" {
		fmt.Fprintln(os.Stderr, "--from and --to are required (ISO 8601 dates, e.g. 2026-09-01)")
		return tp.TimeParams{}, false
	}
	return tp.TimeParams{
		FirstDay:         first,
		LastDay:          last,
		HoursPerDay:      *f.floats["hours-per-day"],
		HalfDayThreshold: *f.floats["half-day-threshold"],
		SortTasks:        f.seen["sort-tasks"] && f.getBool("sort-tasks"),
	}, true
}

func cmdTimesheet(client *tp.Client, args []string) int {
	f := newFlags("timesheet")
	timeFlags(f)
	f.bool("sort-tasks", "Sort tasks alphabetically within each day")
	rest, code, ok := f.parse(args, "<user id, name, email or \"me\">")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "user")
	if !ok {
		return code
	}
	params, ok := timeParams(f)
	if !ok {
		return 2
	}
	id, err := client.ResolveUserID(ref)
	if err != nil {
		return failf("%v", err)
	}
	sheet, err := client.GetUserTimesheet(id, params)
	if err != nil {
		return failf("get timesheet failed: %v", err)
	}
	return out(tools.FormatTimesheet(sheet))
}

func cmdTimeReport(client *tp.Client, args []string) int {
	f := newFlags("time-report")
	timeFlags(f)
	rest, code, ok := f.parse(args, "<user id, name, email or \"me\">")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "user")
	if !ok {
		return code
	}
	params, ok := timeParams(f)
	if !ok {
		return 2
	}
	id, err := client.ResolveUserID(ref)
	if err != nil {
		return failf("%v", err)
	}
	report, err := client.GetUserTimeReport(id, params)
	if err != nil {
		return failf("get time report failed: %v", err)
	}
	return out(tools.FormatTimeReport(report))
}

func cmdUserTasks(client *tp.Client, args []string) int {
	f := newFlags("user-tasks")
	f.str("scope", "all (default) or todo for the current to-do list")
	f.str("project", "todo scope only: restrict to this project (id or name)")
	f.bool("not-closed", "Only tasks that are neither completed nor cancelled")
	f.str("from", "Period start (ISO 8601 date)")
	f.str("to", "Period end (ISO 8601 date)")
	rest, code, ok := f.parse(args, "<user id, name, email or \"me\">")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "user")
	if !ok {
		return code
	}
	id, err := client.ResolveUserID(ref)
	if err != nil {
		return failf("%v", err)
	}

	if strings.EqualFold(f.getStr("scope"), "todo") {
		projectID := 0
		if pref := f.getStr("project"); pref != "" {
			resolved, err := client.ResolveProjectID(pref)
			if err != nil {
				return failf("%v", err)
			}
			projectID = resolved
		}
		tasks, err := client.GetUserTodoList(id, projectID)
		if err != nil {
			return failf("get to-do list failed: %v", err)
		}
		return out(tools.FormatTasks(tasks, fmt.Sprintf("To-do list of user #%d", id)))
	}

	tasks, err := client.ListUserTasks(id, tp.TaskFilter{
		NotClosed:   f.getBool("not-closed"),
		PeriodStart: f.getStr("from"),
		PeriodEnd:   f.getStr("to"),
	})
	if err != nil {
		return failf("list user tasks failed: %v", err)
	}
	return out(tools.FormatTasks(tasks, fmt.Sprintf("Tasks of user #%d", id)))
}

func cmdAssignments(client *tp.Client, args []string) int {
	f := newFlags("assignments")
	f.str("from", "First day of the period (ISO 8601 date, required)")
	f.str("to", "Last day of the period (ISO 8601 date, required)")
	rest, code, ok := f.parse(args, "<user id, name, email or \"me\">")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "user")
	if !ok {
		return code
	}
	first, last := f.getStr("from"), f.getStr("to")
	if first == "" || last == "" {
		return failf("--from and --to are required (ISO 8601 dates)")
	}
	id, err := client.ResolveUserID(ref)
	if err != nil {
		return failf("%v", err)
	}
	assignments, err := client.GetUserAssignments(id, first, last)
	if err != nil {
		return failf("get assignments failed: %v", err)
	}
	return out(tools.FormatAssignments(assignments, id))
}

func cmdListPortfolios(client *tp.Client, args []string) int {
	f := newFlags("list-portfolios")
	f.bool("archived", "Include archived portfolios")
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}
	portfolios, err := client.ListPortfolios(f.getBool("archived"))
	if err != nil {
		return failf("list portfolios failed: %v", err)
	}
	return out(tools.FormatPortfolios(portfolios))
}

func cmdPortfolioReport(client *tp.Client, args []string) int {
	f := newFlags("portfolio-report")
	rest, code, ok := f.parse(args, "<portfolio id or name>")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "portfolio")
	if !ok {
		return code
	}
	id, err := client.ResolvePortfolioID(ref)
	if err != nil {
		return failf("%v", err)
	}
	raw, err := client.PortfolioProgressReport(id)
	if err != nil {
		return failf("get portfolio progress report failed: %v", err)
	}
	return out(tools.FormatJSON(fmt.Sprintf("Portfolio #%d — progress report", id), raw))
}

func cmdReference(client *tp.Client, args []string) int {
	f := newFlags("reference")
	f.str("kind", "obs, profiles, npactivities, projectstates or customforms")
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}
	kind := strings.ToLower(f.getStr("kind"))
	if kind == "" {
		return failf("--kind is required")
	}
	items, err := client.ListReference(kind)
	if err != nil {
		return failf("list %s failed: %v", kind, err)
	}
	return out(tools.FormatReference(kind, items))
}
