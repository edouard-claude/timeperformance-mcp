package cli

import (
	"fmt"
	"net/url"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
	"github.com/edouard-claude/timeperformance-mcp/internal/tools"
)

// toValues builds a query string from a plain map.
func toValues(m map[string]string) url.Values {
	values := url.Values{}
	for k, v := range m {
		values.Set(k, v)
	}
	return values
}

// titleCase upper-cases the first letter, for headings built from a kind.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// yesFlag declares the confirmation flag every write command requires.
func yesFlag(f *flags) {
	f.bool("yes", "Actually send the change (without it, the command only prints what it would do)")
}

func line(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return label + ": " + value
}

func lineInt(label string, value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%s: #%d", label, *value)
}

// --- projects ---

func projectWriteFlags(f *flags) {
	f.str("name", "Project name")
	f.str("description", "Description")
	f.str("sponsor", "Client name")
	f.str("project-type", "Project type")
	f.str("external-id", "External reference")
	f.int("priority", "Priority, 1 (highest) to 5 (lowest)")
	f.int("state", "ProjectState id")
}

func projectWrite(f *flags) tp.ProjectWrite {
	return tp.ProjectWrite{
		Name:        f.optStr("name"),
		Description: f.optStr("description"),
		Sponsor:     f.optStr("sponsor"),
		ProjectType: f.optStr("project-type"),
		ExternalID:  f.optStr("external-id"),
		Priority:    f.optInt("priority"),
		State:       f.optInt("state"),
	}
}

func projectSummary(p tp.ProjectWrite) []string {
	var summary []string
	for _, s := range []string{
		line("Name", strOr(p.Name)),
		line("Client", strOr(p.Sponsor)),
		line("Type", strOr(p.ProjectType)),
		line("Description", strOr(p.Description)),
		lineInt("Priority", p.Priority),
		lineInt("State", p.State),
		lineInt("Project manager", p.ProjectManager),
		lineInt("Template", p.Template),
	} {
		if s != "" {
			summary = append(summary, s)
		}
	}
	return summary
}

func strOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func cmdCreateProject(client *tp.Client, args []string) int {
	f := newFlags("create-project")
	projectWriteFlags(f)
	f.str("project-manager", "Project manager: user id, name or email (required by the back-office API)")
	f.int("phasing-depth", "0 = no phases, 1 = phases (default), 2 = phases and sub-phases")
	f.int("template", "Id of the project template to clone")
	yesFlag(f)
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}

	payload := projectWrite(f)
	payload.PhasingDepth = f.optInt("phasing-depth")
	payload.Template = f.optInt("template")
	if payload.Name == nil || *payload.Name == "" {
		return failf("--name is required")
	}
	if ref := f.getStr("project-manager"); ref != "" {
		id, err := client.ResolveUserID(ref)
		if err != nil {
			return failf("project manager: %v", err)
		}
		payload.ProjectManager = &id
	}

	if !confirmed(f, "Create a TimePerformance project:", projectSummary(payload)) {
		return 0
	}
	project, err := client.CreateProject(payload)
	if err != nil {
		return failf("create project failed: %v", err)
	}
	return out("Project created.\n\n" + tools.FormatProject(project))
}

func cmdUpdateProject(client *tp.Client, args []string) int {
	f := newFlags("update-project")
	projectWriteFlags(f)
	yesFlag(f)
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

	payload := projectWrite(f)
	summary := projectSummary(payload)
	if len(summary) == 0 {
		return failf("nothing to update: pass at least one field to change")
	}
	if !confirmed(f, fmt.Sprintf("Update project #%d:", id), summary) {
		return 0
	}
	project, err := client.UpdateProject(id, payload)
	if err != nil {
		return failf("update project failed: %v", err)
	}
	return out("Project updated.\n\n" + tools.FormatProject(project))
}

// --- tasks ---

func taskWriteFlags(f *flags) {
	f.str("name", "Task name")
	f.str("description", "Description")
	f.str("due-date", "Due date (ISO 8601 date)")
	f.str("performer", "Assigned user: id, name, email or \"me\"")
	f.int("deliverable", "Deliverable id the task contributes to")
	f.int("task-type", "TaskDefinition id")
	f.str("external-id", "External reference")
}

func taskWrite(client *tp.Client, f *flags) (tp.TaskWrite, bool) {
	payload := tp.TaskWrite{
		Name:        f.optStr("name"),
		Description: f.optStr("description"),
		DueDate:     f.optStr("due-date"),
		Goal:        f.optInt("deliverable"),
		TaskType:    f.optInt("task-type"),
		ExternalID:  f.optStr("external-id"),
	}
	if ref := f.getStr("performer"); ref != "" {
		id, err := client.ResolveUserID(ref)
		if err != nil {
			failf("performer: %v", err)
			return payload, false
		}
		payload.Performer = &id
	}
	return payload, true
}

func taskSummary(p tp.TaskWrite) []string {
	var summary []string
	for _, s := range []string{
		line("Name", strOr(p.Name)),
		line("Description", strOr(p.Description)),
		line("Due date", strOr(p.DueDate)),
		lineInt("Performer", p.Performer),
		lineInt("Deliverable", p.Goal),
		lineInt("Task type", p.TaskType),
	} {
		if s != "" {
			summary = append(summary, s)
		}
	}
	return summary
}

func cmdCreateTask(client *tp.Client, args []string) int {
	f := newFlags("create-task")
	f.str("project", "Project id or name (required)")
	taskWriteFlags(f)
	yesFlag(f)
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}
	if f.getStr("project") == "" {
		return failf("--project is required")
	}
	projectID, err := client.ResolveProjectID(f.getStr("project"))
	if err != nil {
		return failf("%v", err)
	}
	payload, ok := taskWrite(client, f)
	if !ok {
		return 1
	}
	if payload.Name == nil && payload.TaskType == nil {
		return failf("--name is required unless --task-type is given")
	}

	summary := append([]string{fmt.Sprintf("Project: #%d", projectID)}, taskSummary(payload)...)
	if !confirmed(f, "Create a TimePerformance task:", summary) {
		return 0
	}
	task, err := client.CreateTask(projectID, payload)
	if err != nil {
		return failf("create task failed: %v", err)
	}
	return out("Task created.\n\n" + tools.FormatTask(task))
}

func cmdUpdateTask(client *tp.Client, args []string) int {
	f := newFlags("update-task")
	taskWriteFlags(f)
	yesFlag(f)
	rest, code, ok := f.parse(args, "<task id>")
	if !ok {
		return code
	}
	id, code, ok := firstIntArg(rest, "task id")
	if !ok {
		return code
	}
	payload, ok := taskWrite(client, f)
	if !ok {
		return 1
	}
	summary := taskSummary(payload)
	if len(summary) == 0 {
		return failf("nothing to update: pass at least one field to change")
	}
	if !confirmed(f, fmt.Sprintf("Update task #%d:", id), summary) {
		return 0
	}
	task, err := client.UpdateTask(id, payload)
	if err != nil {
		return failf("update task failed: %v", err)
	}
	return out("Task updated.\n\n" + tools.FormatTask(task))
}

// --- deliverables & phases ---

func wbsWriteFlags(f *flags, deliverable bool) {
	f.str("name", "Name")
	f.str("description", "Description")
	f.str("state", "State: draft, ready, open, closed or cancelled")
	f.str("first-day", "Planned start date (ISO 8601 date)")
	f.str("last-day", "Planned end date (ISO 8601 date)")
	f.bool("milestone", "Mark as a milestone")
	f.bool("locked-schedule", "Lock the schedule dates")
	f.str("external-id", "External reference")
	if deliverable {
		f.str("progress-mode", "MANUAL, TASK_RTD, TASK_COUNT, SCHEDULE, WORKLOAD or WEIGHTED_SUM")
		f.str("acceptance", "Acceptance criteria")
		f.int("phase", "Phase id the deliverable is planned for")
	}
}

func wbsWrite(f *flags, deliverable bool) tp.WBSWrite {
	w := tp.WBSWrite{
		Name:           f.optStr("name"),
		Description:    f.optStr("description"),
		State:          f.optStr("state"),
		FirstDay:       f.optStr("first-day"),
		LastDay:        f.optStr("last-day"),
		Milestone:      f.optBool("milestone"),
		LockedSchedule: f.optBool("locked-schedule"),
		ExternalID:     f.optStr("external-id"),
	}
	if deliverable {
		w.ProgressMode = f.optStr("progress-mode")
		w.Acceptance = f.optStr("acceptance")
		w.Iteration = f.optInt("phase")
	}
	return w
}

func wbsSummary(w tp.WBSWrite) []string {
	var summary []string
	for _, s := range []string{
		line("Name", strOr(w.Name)),
		line("Description", strOr(w.Description)),
		line("State", strOr(w.State)),
		line("Planned start", strOr(w.FirstDay)),
		line("Planned end", strOr(w.LastDay)),
		line("Progress mode", strOr(w.ProgressMode)),
		line("Acceptance criteria", strOr(w.Acceptance)),
		lineInt("Phase", w.Iteration),
	} {
		if s != "" {
			summary = append(summary, s)
		}
	}
	if w.Milestone != nil {
		summary = append(summary, fmt.Sprintf("Milestone: %t", *w.Milestone))
	}
	if w.LockedSchedule != nil {
		summary = append(summary, fmt.Sprintf("Locked schedule: %t", *w.LockedSchedule))
	}
	return summary
}

func cmdCreateWBS(client *tp.Client, args []string, deliverable bool) int {
	kind := "phase"
	if deliverable {
		kind = "deliverable"
	}
	f := newFlags("create-" + kind)
	f.str("project", "Project id or name (for a top-level element)")
	f.int("parent", "Parent "+kind+" id (creates a sub-"+kind+" instead)")
	wbsWriteFlags(f, deliverable)
	yesFlag(f)
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}

	payload := wbsWrite(f, deliverable)
	if payload.Name == nil || *payload.Name == "" {
		return failf("--name is required")
	}
	parentID := f.getInt("parent")
	if parentID == 0 && f.getStr("project") == "" {
		return failf("pass --project (top-level) or --parent (sub-%s)", kind)
	}

	if parentID > 0 {
		summary := append([]string{fmt.Sprintf("Parent %s: #%d", kind, parentID)}, wbsSummary(payload)...)
		if !confirmed(f, fmt.Sprintf("Create a TimePerformance sub-%s:", kind), summary) {
			return 0
		}
		var (
			item *tp.WBSElement
			err  error
		)
		if deliverable {
			item, err = client.CreateSubDeliverable(parentID, payload)
		} else {
			item, err = client.CreateSubPhase(parentID, payload)
		}
		if err != nil {
			return failf("create sub-%s failed: %v", kind, err)
		}
		return out(fmt.Sprintf("Sub-%s created.\n\n", kind) + tools.FormatWBSElement(item, titleCase(kind)))
	}

	projectID, err := client.ResolveProjectID(f.getStr("project"))
	if err != nil {
		return failf("%v", err)
	}
	summary := append([]string{fmt.Sprintf("Project: #%d", projectID)}, wbsSummary(payload)...)
	if !confirmed(f, fmt.Sprintf("Create a TimePerformance %s:", kind), summary) {
		return 0
	}
	var item *tp.WBSElement
	if deliverable {
		item, err = client.CreateDeliverable(projectID, payload)
	} else {
		item, err = client.CreatePhase(projectID, payload)
	}
	if err != nil {
		return failf("create %s failed: %v", kind, err)
	}
	return out(fmt.Sprintf("%s created.\n\n", titleCase(kind)) + tools.FormatWBSElement(item, titleCase(kind)))
}

func cmdUpdateWBS(client *tp.Client, args []string, deliverable bool) int {
	kind := "phase"
	if deliverable {
		kind = "deliverable"
	}
	f := newFlags("update-" + kind)
	wbsWriteFlags(f, deliverable)
	yesFlag(f)
	rest, code, ok := f.parse(args, "<"+kind+" id>")
	if !ok {
		return code
	}
	id, code, ok := firstIntArg(rest, kind+" id")
	if !ok {
		return code
	}

	payload := wbsWrite(f, deliverable)
	summary := wbsSummary(payload)
	if len(summary) == 0 {
		return failf("nothing to update: pass at least one field to change")
	}
	if !confirmed(f, fmt.Sprintf("Update %s #%d:", kind, id), summary) {
		return 0
	}

	var (
		item *tp.WBSElement
		err  error
	)
	if deliverable {
		item, err = client.UpdateDeliverable(id, payload)
	} else {
		item, err = client.UpdatePhase(id, payload)
	}
	if err != nil {
		return failf("update %s failed: %v", kind, err)
	}
	return out(fmt.Sprintf("%s updated.\n\n", titleCase(kind)) + tools.FormatWBSElement(item, titleCase(kind)))
}

// --- expenses ---

func expenseWriteFlags(f *flags) {
	f.str("name", "Expense label")
	f.str("description", "Description")
	f.float("amount", "Monetary amount")
	f.str("currency", "Currency code of the amount (e.g. EUR)")
	f.str("date", "Expense date (ISO 8601 date)")
	f.str("counterparty", "Supplier or counterparty")
	f.bool("smoothed", "Smooth the expense over time")
	f.int("deliverable", "Deliverable id the expense is attached to")
	f.str("external-id", "External reference")
}

func expenseWrite(f *flags) (tp.ExpenseWrite, bool) {
	w := tp.ExpenseWrite{
		Name:         f.optStr("name"),
		Description:  f.optStr("description"),
		Date:         f.optStr("date"),
		Counterparty: f.optStr("counterparty"),
		Smoothed:     f.optBool("smoothed"),
		Deliverable:  f.optInt("deliverable"),
		ExternalID:   f.optStr("external-id"),
	}
	if amount := f.optFloat("amount"); amount != nil {
		currency := f.getStr("currency")
		if currency == "" {
			failf("--currency is required when --amount is set (e.g. EUR)")
			return w, false
		}
		w.Amount = &tp.UnitValue{Val: *amount, Unit: currency}
	}
	return w, true
}

func expenseSummary(w tp.ExpenseWrite) []string {
	var summary []string
	for _, s := range []string{
		line("Label", strOr(w.Name)),
		line("Description", strOr(w.Description)),
		line("Date", strOr(w.Date)),
		line("Counterparty", strOr(w.Counterparty)),
		lineInt("Deliverable", w.Deliverable),
	} {
		if s != "" {
			summary = append(summary, s)
		}
	}
	if w.Amount != nil {
		summary = append(summary, fmt.Sprintf("Amount: %g %s", w.Amount.Val, w.Amount.Unit))
	}
	if w.Smoothed != nil {
		summary = append(summary, fmt.Sprintf("Smoothed: %t", *w.Smoothed))
	}
	return summary
}

func cmdCreateExpense(client *tp.Client, args []string) int {
	f := newFlags("create-expense")
	f.str("project", "Project id or name (required)")
	expenseWriteFlags(f)
	yesFlag(f)
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}
	if f.getStr("project") == "" {
		return failf("--project is required")
	}
	projectID, err := client.ResolveProjectID(f.getStr("project"))
	if err != nil {
		return failf("%v", err)
	}
	payload, ok := expenseWrite(f)
	if !ok {
		return 1
	}
	if payload.Name == nil || *payload.Name == "" {
		return failf("--name is required")
	}

	summary := append([]string{fmt.Sprintf("Project: #%d", projectID)}, expenseSummary(payload)...)
	if !confirmed(f, "Record a TimePerformance expense:", summary) {
		return 0
	}
	expense, err := client.CreateExpense(projectID, payload)
	if err != nil {
		return failf("create expense failed: %v", err)
	}
	return out("Expense created.\n\n" + tools.FormatExpense(expense))
}

func cmdUpdateExpense(client *tp.Client, args []string) int {
	f := newFlags("update-expense")
	expenseWriteFlags(f)
	yesFlag(f)
	rest, code, ok := f.parse(args, "<expense id>")
	if !ok {
		return code
	}
	id, code, ok := firstIntArg(rest, "expense id")
	if !ok {
		return code
	}
	payload, ok := expenseWrite(f)
	if !ok {
		return 1
	}
	summary := expenseSummary(payload)
	if len(summary) == 0 {
		return failf("nothing to update: pass at least one field to change")
	}
	if !confirmed(f, fmt.Sprintf("Update expense #%d:", id), summary) {
		return 0
	}
	expense, err := client.UpdateExpense(id, payload)
	if err != nil {
		return failf("update expense failed: %v", err)
	}
	return out("Expense updated.\n\n" + tools.FormatExpense(expense))
}

// --- users ---

func userWriteFlags(f *flags) {
	f.str("firstname", "First name")
	f.str("lastname", "Last name")
	f.str("email", "Email address")
	f.str("external-id", "External reference")
	f.str("availability-start", "Start of the availability period (ISO 8601 date)")
	f.str("availability-end", "End of the availability period (ISO 8601 date)")
	f.int("obs", "Organizational unit id")
	f.int("profile", "Skill profile id")
	f.str("locale", "Locale code, e.g. fr")
	f.str("gender", "Gender as accepted by the API")
	f.str("sso-uid", "Single Sign-On identifier")
	f.bool("extern", "External collaborator")
	f.bool("app-access", "Right: access to the application")
	f.bool("administrator", "Right: administrator")
	f.bool("manager", "Right: manager")
	f.bool("pmo", "Right: PMO")
	f.bool("limited-access", "Right: limited access")
	f.bool("create-project", "Right: create projects")
	f.bool("manage-self", "Right: manage own assignments")
}

func userWrite(f *flags) tp.UserWrite {
	w := tp.UserWrite{
		Firstname:         f.optStr("firstname"),
		Lastname:          f.optStr("lastname"),
		Email:             f.optStr("email"),
		ExternalID:        f.optStr("external-id"),
		AvailabilityStart: f.optStr("availability-start"),
		AvailabilityEnd:   f.optStr("availability-end"),
		OBS:               f.optInt("obs"),
		Profile:           f.optInt("profile"),
		Locale:            f.optStr("locale"),
		Gender:            f.optStr("gender"),
		SSOUID:            f.optStr("sso-uid"),
		Extern:            f.optBool("extern"),
	}
	rights := tp.UserRightsWrite{
		AppAccess:     f.optBool("app-access"),
		Administrator: f.optBool("administrator"),
		Manager:       f.optBool("manager"),
		PMO:           f.optBool("pmo"),
		LimitedAccess: f.optBool("limited-access"),
		CreateProject: f.optBool("create-project"),
		ManageSelf:    f.optBool("manage-self"),
	}
	if rights != (tp.UserRightsWrite{}) {
		w.Rights = &rights
	}
	return w
}

func userSummary(w tp.UserWrite) []string {
	var summary []string
	for _, s := range []string{
		line("First name", strOr(w.Firstname)),
		line("Last name", strOr(w.Lastname)),
		line("Email", strOr(w.Email)),
		line("Availability start", strOr(w.AvailabilityStart)),
		line("Availability end", strOr(w.AvailabilityEnd)),
		lineInt("Organizational unit", w.OBS),
		lineInt("Skill profile", w.Profile),
	} {
		if s != "" {
			summary = append(summary, s)
		}
	}
	if w.Extern != nil {
		summary = append(summary, fmt.Sprintf("External collaborator: %t", *w.Extern))
	}
	if r := w.Rights; r != nil {
		var changed []string
		for _, pair := range []struct {
			label string
			value *bool
		}{
			{"app access", r.AppAccess}, {"administrator", r.Administrator}, {"manager", r.Manager},
			{"pmo", r.PMO}, {"limited access", r.LimitedAccess}, {"create project", r.CreateProject},
			{"manage self", r.ManageSelf},
		} {
			if pair.value != nil {
				changed = append(changed, fmt.Sprintf("%s=%t", pair.label, *pair.value))
			}
		}
		summary = append(summary, "Rights: "+strings.Join(changed, ", "))
	}
	return summary
}

func cmdCreateUser(client *tp.Client, args []string) int {
	f := newFlags("create-user")
	userWriteFlags(f)
	yesFlag(f)
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}
	payload := userWrite(f)
	if payload.Firstname == nil || payload.Lastname == nil {
		return failf("--firstname and --lastname are required")
	}
	if !confirmed(f, "Create a TimePerformance user account:", userSummary(payload)) {
		return 0
	}
	user, err := client.CreateUser(payload)
	if err != nil {
		return failf("create user failed: %v", err)
	}
	return out("User created.\n\n" + tools.FormatUser(user))
}

func cmdUpdateUser(client *tp.Client, args []string) int {
	f := newFlags("update-user")
	userWriteFlags(f)
	yesFlag(f)
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
	payload := userWrite(f)
	summary := userSummary(payload)
	if len(summary) == 0 {
		return failf("nothing to update: pass at least one field to change")
	}
	if !confirmed(f, fmt.Sprintf("Update user #%d:", id), summary) {
		return 0
	}
	user, err := client.UpdateUser(id, payload)
	if err != nil {
		return failf("update user failed: %v", err)
	}
	return out("User updated.\n\n" + tools.FormatUser(user))
}

func cmdArchiveUser(client *tp.Client, args []string) int {
	f := newFlags("archive-user")
	yesFlag(f)
	rest, code, ok := f.parse(args, "<user id, name or email>")
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
	label := fmt.Sprintf("User: #%d", id)
	if user, err := client.GetUser(id); err == nil {
		label = fmt.Sprintf("User: %s (#%d, %s)", user.Name, user.ID, user.Email)
	}
	if !confirmed(f, "Archive a TimePerformance user account:", []string{label}) {
		return 0
	}
	if err := client.ArchiveUser(id); err != nil {
		return failf("archive user failed: %v", err)
	}
	return out(fmt.Sprintf("User #%d archived.", id))
}

// --- team & deletion ---

func cmdTeamMember(client *tp.Client, args []string) int {
	f := newFlags("team-member")
	f.str("user", "User id, name, email or \"me\" (required)")
	f.str("action", "add, set_rights or remove (required)")
	f.bool("project-manager", "Grant the project-manager right (add and set_rights)")
	f.int("profile", "add only: skill profile id for this member")
	yesFlag(f)
	rest, code, ok := f.parse(args, "<project id or name>")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "project")
	if !ok {
		return code
	}
	action := strings.ToLower(f.getStr("action"))
	switch action {
	case "add", "set_rights", "remove":
	default:
		return failf("--action must be add, set_rights or remove")
	}
	if f.getStr("user") == "" {
		return failf("--user is required")
	}

	projectID, err := client.ResolveProjectID(ref)
	if err != nil {
		return failf("%v", err)
	}
	userID, err := client.ResolveUserID(f.getStr("user"))
	if err != nil {
		return failf("%v", err)
	}

	summary := []string{
		fmt.Sprintf("Project: #%d", projectID),
		fmt.Sprintf("User: #%d", userID),
		"Action: " + action,
	}
	if action != "remove" {
		summary = append(summary, fmt.Sprintf("Project manager right: %t", f.getBool("project-manager")))
	}
	if !confirmed(f, "Change a TimePerformance project team:", summary) {
		return 0
	}

	switch action {
	case "add":
		err = client.AddTeamMember(projectID, userID, f.getInt("profile"), f.getBool("project-manager"))
	case "set_rights":
		err = client.SetTeamMemberRights(projectID, userID, f.getBool("project-manager"))
	case "remove":
		err = client.RemoveTeamMember(projectID, userID)
	}
	if err != nil {
		return failf("%s failed: %v", action, err)
	}
	return out(fmt.Sprintf("Done: %s user #%d on project #%d.", action, userID, projectID))
}

func cmdDelete(client *tp.Client, args []string) int {
	f := newFlags("delete")
	f.str("kind", "task, deliverable, phase, expense or datasheet (required)")
	f.int("id", "Numeric id of the element to delete (required)")
	yesFlag(f)
	if _, code, ok := f.parse(args, ""); !ok {
		return code
	}
	kind := strings.ToLower(f.getStr("kind"))
	if _, ok := tp.DeletePath[kind]; !ok {
		return failf("--kind must be one of: task, deliverable, phase, expense, datasheet")
	}
	id := f.getInt("id")
	if id == 0 {
		return failf("--id is required")
	}

	summary := []string{
		fmt.Sprintf("Target: %s #%d", kind, id),
		"This deletion is permanent.",
	}
	if !confirmed(f, "Delete a TimePerformance element:", summary) {
		return 0
	}
	if err := client.DeleteElement(kind, id); err != nil {
		return failf("delete %s #%d failed: %v", kind, id, err)
	}
	return out(fmt.Sprintf("Deleted %s #%d.", kind, id))
}

// --- leave ---

func cmdLeave(client *tp.Client, args []string) int {
	f := newFlags("leave")
	f.str("type", "Leave type: unavailability id or name (required unless --remove)")
	f.str("from", "First day of the leave (ISO 8601 date, required)")
	f.str("to", "Last day of the leave, inclusive (ISO 8601 date, required)")
	f.str("from-half", "AM (default) or PM: PM starts the leave at noon")
	f.str("to-half", "PM (default) or AM: AM ends the leave at noon")
	f.bool("weekends", "Also plan Saturdays and Sundays")
	f.bool("remove", "Clear the leave on these half-days instead of adding it")
	f.str("mode", "On conflict with existing assignments: ALL_OR_NOTHING (default), NON_CONFLICT_ONLY, FORCE_OVERRIDE")
	yesFlag(f)
	rest, code, ok := f.parse(args, "<user id, name, email or \"me\">")
	if !ok {
		return code
	}
	ref, code, ok := firstArg(rest, "user")
	if !ok {
		return code
	}
	if f.getStr("from") == "" || f.getStr("to") == "" {
		return failf("--from and --to are required (ISO 8601 dates)")
	}
	userID, err := client.ResolveUserID(ref)
	if err != nil {
		return failf("%v", err)
	}

	remove := f.getBool("remove")
	plan, activity, err := client.PlanUserLeave(tp.LeaveRequest{
		UserID:    userID,
		Activity:  f.getStr("type"),
		FirstDay:  f.getStr("from"),
		FirstHalf: f.getStr("from-half"),
		LastDay:   f.getStr("to"),
		LastHalf:  f.getStr("to-half"),
		Weekends:  f.getBool("weekends"),
		Remove:    remove,
		SyncMode:  f.getStr("mode"),
	})
	if err != nil {
		return failf("%v", err)
	}
	if len(plan.Added) == 0 && len(plan.Removed) == 0 {
		return out(tools.NoLeaveChange)
	}
	if !confirmed(f, "Change a TimePerformance leave schedule:", tools.LeaveSummary(plan, activity, userID, remove)) {
		return 0
	}
	res, err := client.SyncUserUnavailabilities(userID, plan.Sync)
	if err != nil {
		return failf("sync unavailabilities failed: %v", err)
	}
	return out(tools.FormatSyncResult(res, plan.Sync.SyncMode))
}
