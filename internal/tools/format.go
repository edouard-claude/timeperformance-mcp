package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	tp "github.com/edouard-claude/timeperformance-mcp/internal/timeperformance"
)

// maxJSON caps a raw report echoed back to the caller.
const maxJSON = 60000

// FormatJSON pretty-prints a raw report, truncated to stay usable in a chat.
func FormatJSON(title string, raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		buf.Reset()
		buf.Write(raw)
	}
	out := buf.String()
	truncated := ""
	if len(out) > maxJSON {
		out = out[:maxJSON]
		truncated = fmt.Sprintf("\n… [truncated, %d bytes total]", buf.Len())
	}
	return fmt.Sprintf("# %s\n\n```json\n%s\n```%s\n", title, out, truncated)
}

// trimFloat renders a float rounded to two decimals, without trailing zeros:
// the API returns hours as raw fractions (17.16638888888889).
func trimFloat(f float64) string {
	return strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64)
}

// entityLabel renders an optional entity reference as "Name (#id)".
func entityLabel(e *tp.Entity) string {
	if e == nil || e.ID == 0 {
		return ""
	}
	name := e.Name
	if name == "" {
		name = e.Path
	}
	if name == "" {
		return fmt.Sprintf("#%d", e.ID)
	}
	return fmt.Sprintf("%s (#%d)", name, e.ID)
}

func appendField(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", label, value)
}

// --- Projects ---

// FormatProjects renders a project listing.
func FormatProjects(projects []tp.Project, title string) string {
	if len(projects) == 0 {
		return "No project found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%d)\n\n", title, len(projects))
	for _, p := range projects {
		fmt.Fprintf(&b, "- #%d %s", p.ID, p.Name)
		if p.State != nil && p.State.Name != "" {
			fmt.Fprintf(&b, " | state: %s", p.State.Name)
		}
		if p.Sponsor != "" {
			fmt.Fprintf(&b, " | client: %s", p.Sponsor)
		}
		if p.ProjectType != "" {
			fmt.Fprintf(&b, " | type: %s", p.ProjectType)
		}
		if p.Archived {
			b.WriteString(" | archived")
		}
		if p.ExternalID != "" {
			fmt.Fprintf(&b, " | ext: %s", p.ExternalID)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FormatProject renders the details of one project.
func FormatProject(p *tp.Project) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Project #%d — %s\n", p.ID, p.Name)
	if p.State != nil {
		state := p.State.Name
		if p.State.Finished {
			state += " (end state)"
		}
		appendField(&b, "State", state)
	}
	appendField(&b, "Client", p.Sponsor)
	appendField(&b, "Type", p.ProjectType)
	if p.Priority > 0 {
		appendField(&b, "Priority", fmt.Sprintf("%d (1 = highest)", p.Priority))
	}
	appendField(&b, "Created", p.CreationDate)
	appendField(&b, "External id", p.ExternalID)
	if p.Template {
		appendField(&b, "Template", "yes")
	}
	if p.Archived {
		appendField(&b, "Archived", "yes")
	}
	if len(p.Labels) > 0 {
		names := make([]string, 0, len(p.Labels))
		for _, l := range p.Labels {
			names = append(names, l.Name)
		}
		appendField(&b, "Labels", strings.Join(names, ", "))
	}
	if p.Description != "" {
		fmt.Fprintf(&b, "\n## Description\n%s\n", p.Description)
	}
	return b.String()
}

// --- Tasks ---

// FormatTasks renders a task listing.
func FormatTasks(tasks []tp.Task, title string) string {
	if len(tasks) == 0 {
		return "No task found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%d)\n\n", title, len(tasks))
	for _, t := range tasks {
		fmt.Fprintf(&b, "- #%d %s", t.ID, t.Name)
		if t.State != "" {
			fmt.Fprintf(&b, " | %s", t.State)
		}
		if t.Closed {
			b.WriteString(" (closed)")
		}
		if label := entityLabel(t.Project); label != "" {
			fmt.Fprintf(&b, " | project: %s", label)
		}
		if t.Performer != nil && t.Performer.Name != "" {
			fmt.Fprintf(&b, " | performer: %s", t.Performer.Name)
		}
		if t.DueDate != "" {
			fmt.Fprintf(&b, " | due: %s", t.DueDate)
		}
		if t.Effort > 0 || t.RTD > 0 {
			fmt.Fprintf(&b, " | done %sh / left %sh", trimFloat(t.Effort), trimFloat(t.RTD))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FormatTask renders the details of one task.
func FormatTask(t *tp.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Task #%d — %s\n", t.ID, t.Name)
	appendField(&b, "State", t.State)
	if t.Closed {
		appendField(&b, "Closed", "yes")
	}
	appendField(&b, "Project", entityLabel(t.Project))
	appendField(&b, "Phase", entityLabel(t.Phase))
	appendField(&b, "Iteration", entityLabel(t.Iteration))
	appendField(&b, "Deliverable", entityLabel(t.Goal))
	appendField(&b, "Task type", entityLabel(t.TaskType))
	appendField(&b, "Category", entityLabel(t.TaskCategory))
	if t.Performer != nil {
		appendField(&b, "Performer", fmt.Sprintf("%s (#%d)", t.Performer.Name, t.Performer.ID))
	}
	appendField(&b, "Due date", t.DueDate)
	appendField(&b, "Start", t.Start)
	appendField(&b, "End", t.End)
	if t.Effort > 0 || t.RTD > 0 {
		appendField(&b, "Effort", fmt.Sprintf("%sh done, %sh remaining", trimFloat(t.Effort), trimFloat(t.RTD)))
	}
	appendField(&b, "External id", t.ExternalID)
	if t.Description != "" {
		fmt.Fprintf(&b, "\n## Description\n%s\n", t.Description)
	}
	if len(t.Steps) > 0 {
		b.WriteString("\n## Check-list\n")
		for _, s := range t.Steps {
			mark := " "
			if s.Done {
				mark = "x"
			}
			fmt.Fprintf(&b, "- [%s] %s\n", mark, s.Name)
		}
	}
	return b.String()
}

// --- WBS (deliverables & phases) ---

// FormatWBSTree renders a deliverables or phases tree.
func FormatWBSTree(elements []tp.WBSElement, title string) string {
	if len(elements) == 0 {
		return "No element found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	var walk func(items []tp.WBSElement, depth int)
	walk = func(items []tp.WBSElement, depth int) {
		for _, e := range items {
			fmt.Fprintf(&b, "%s- #%d %s", strings.Repeat("  ", depth), e.ID, e.Name)
			var meta []string
			if e.State != "" {
				meta = append(meta, e.State)
			}
			if e.Milestone != nil && *e.Milestone {
				meta = append(meta, "milestone")
			}
			if e.FirstDay != "" || e.LastDay != "" {
				meta = append(meta, strings.TrimSpace(e.FirstDay+" → "+e.LastDay))
			}
			if e.ProgressMode != "" {
				meta = append(meta, "progress: "+e.ProgressMode)
			}
			if len(meta) > 0 {
				fmt.Fprintf(&b, " | %s", strings.Join(meta, " | "))
			}
			b.WriteString("\n")
			walk(e.Children, depth+1)
		}
	}
	walk(elements, 0)
	return b.String()
}

// FormatWBSElement renders one deliverable or phase.
func FormatWBSElement(e *tp.WBSElement, kind string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s #%d — %s\n", kind, e.ID, e.Name)
	appendField(&b, "Path", e.Path)
	appendField(&b, "State", e.State)
	if e.Milestone != nil {
		appendField(&b, "Milestone", strconv.FormatBool(*e.Milestone))
	}
	if e.LockedSchedule != nil {
		appendField(&b, "Locked schedule", strconv.FormatBool(*e.LockedSchedule))
	}
	appendField(&b, "Planned start", e.FirstDay)
	appendField(&b, "Planned end", e.LastDay)
	appendField(&b, "Progress mode", e.ProgressMode)
	appendField(&b, "Phase", entityLabel(e.Iteration))
	appendField(&b, "Parent phase", entityLabel(e.Phase))
	appendField(&b, "External id", e.ExternalID)
	if e.Description != "" {
		fmt.Fprintf(&b, "\n## Description\n%s\n", e.Description)
	}
	if e.Acceptance != "" {
		fmt.Fprintf(&b, "\n## Acceptance criteria\n%s\n", e.Acceptance)
	}
	if len(e.Children) > 0 {
		fmt.Fprintf(&b, "\n## Children (%d)\n", len(e.Children))
		for _, ch := range e.Children {
			fmt.Fprintf(&b, "- #%d %s\n", ch.ID, ch.Name)
		}
	}
	return b.String()
}

// --- Expenses ---

func amountLabel(a *tp.UnitValue) string {
	if a == nil {
		return ""
	}
	return strings.TrimSpace(trimFloat(a.Val) + " " + a.Unit)
}

// FormatExpenses renders an expense listing.
func FormatExpenses(expenses []tp.Expense, title string) string {
	if len(expenses) == 0 {
		return "No expense found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%d)\n\n", title, len(expenses))
	for _, e := range expenses {
		fmt.Fprintf(&b, "- #%d %s", e.ID, e.Name)
		if amt := amountLabel(e.Amount); amt != "" {
			fmt.Fprintf(&b, " | %s", amt)
		}
		if e.Date != "" {
			fmt.Fprintf(&b, " | %s", e.Date)
		}
		if e.Counterparty != "" {
			fmt.Fprintf(&b, " | %s", e.Counterparty)
		}
		if e.Settled {
			b.WriteString(" | settled")
		}
		if label := entityLabel(e.Deliverable); label != "" {
			fmt.Fprintf(&b, " | deliverable: %s", label)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FormatExpense renders one expense.
func FormatExpense(e *tp.Expense) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Expense #%d — %s\n", e.ID, e.Name)
	appendField(&b, "Amount", amountLabel(e.Amount))
	appendField(&b, "Date", e.Date)
	appendField(&b, "Counterparty", e.Counterparty)
	appendField(&b, "Settled", strconv.FormatBool(e.Settled))
	appendField(&b, "Smoothed", strconv.FormatBool(e.Smoothed))
	appendField(&b, "Deliverable", entityLabel(e.Deliverable))
	appendField(&b, "Phase", entityLabel(e.Phase))
	appendField(&b, "External id", e.ExternalID)
	if e.Description != "" {
		fmt.Fprintf(&b, "\n## Description\n%s\n", e.Description)
	}
	return b.String()
}

// --- Users & team ---

// FormatUsers renders a user listing.
func FormatUsers(users []tp.User, title string) string {
	if len(users) == 0 {
		return "No user found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%d)\n\n", title, len(users))
	for _, u := range users {
		fmt.Fprintf(&b, "- #%d %s", u.ID, u.Name)
		if u.Email != "" {
			fmt.Fprintf(&b, " | %s", u.Email)
		}
		if u.Profile != nil && u.Profile.Name != "" {
			fmt.Fprintf(&b, " | profile: %s", u.Profile.Name)
		}
		if u.OBS != nil && u.OBS.Name != "" {
			fmt.Fprintf(&b, " | unit: %s", u.OBS.Name)
		}
		if u.Extern {
			b.WriteString(" | external")
		}
		if u.Archived {
			b.WriteString(" | archived")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FormatUser renders one user account.
func FormatUser(u *tp.User) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# User #%d — %s\n", u.ID, u.Name)
	appendField(&b, "Email", u.Email)
	appendField(&b, "First/last name", strings.TrimSpace(u.Firstname+" "+u.Lastname))
	appendField(&b, "Unit (OBS)", entityLabel(u.OBS))
	appendField(&b, "Skill profile", entityLabel(u.Profile))
	appendField(&b, "Availability", strings.TrimSpace(u.AvailabilityStart+" → "+u.AvailabilityEnd))
	if u.Extern {
		appendField(&b, "External collaborator", "yes")
	}
	if u.Archived {
		appendField(&b, "Archived", "yes")
	}
	appendField(&b, "Locale", u.Locale)
	appendField(&b, "External id", u.ExternalID)
	if u.CostRate != nil {
		appendField(&b, "Cost rate", strings.TrimSpace(amountLabel(&u.CostRate.UnitValue)+" "+u.CostRate.RateMode))
	}
	if r := u.Rights; r != nil {
		var granted []string
		for label, on := range map[string]bool{
			"app access": r.AppAccess, "administrator": r.Administrator, "manager": r.Manager,
			"pmo": r.PMO, "limited access": r.LimitedAccess, "create project": r.CreateProject,
			"manage self": r.ManageSelf,
		} {
			if on {
				granted = append(granted, label)
			}
		}
		sort.Strings(granted)
		appendField(&b, "Rights", strings.Join(granted, ", "))
	}
	return b.String()
}

// FormatTeam renders the members of a project.
func FormatTeam(members []tp.TeamMember, projectID int) string {
	if len(members) == 0 {
		return fmt.Sprintf("No team member on project #%d.", projectID)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Team of project #%d (%d members)\n\n", projectID, len(members))
	for _, m := range members {
		fmt.Fprintf(&b, "- #%d %s", m.User.ID, m.User.Name)
		if m.User.Email != "" {
			fmt.Fprintf(&b, " | %s", m.User.Email)
		}
		switch m.Rights {
		case "PROJECT_MANAGER":
			b.WriteString(" | project manager")
		case "":
			b.WriteString(" | no access")
		default:
			fmt.Fprintf(&b, " | %s", strings.ToLower(m.Rights))
		}
		if m.Profile != nil && m.Profile.Name != "" {
			fmt.Fprintf(&b, " | profile: %s", m.Profile.Name)
		}
		if m.CostRate != nil && m.CostRate.Val > 0 {
			fmt.Fprintf(&b, " | rate: %s %s", amountLabel(&m.CostRate.UnitValue), m.CostRate.RateMode)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// --- Time ---

func sumEntries(entries []tp.DailyTimeEntry) float64 {
	var total float64
	for _, e := range entries {
		total += e.Hours
	}
	return total
}

func formatEntries(entries []tp.DailyTimeEntry) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Hours == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %sh", e.Day, trimFloat(e.Hours)))
	}
	return strings.Join(parts, ", ")
}

// FormatTimesheet renders the per-task timesheet of a user.
func FormatTimesheet(ts *tp.Timesheet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Timesheet — %s (#%d)\n", ts.User.Name, ts.User.ID)
	fmt.Fprintf(&b, "Period: %s → %s | Total: %sh\n", ts.FirstDay, ts.LastDay, trimFloat(sumEntries(ts.Total)))

	for _, p := range ts.TimeByProjects {
		var projectTotal float64
		for _, t := range p.TimeByTasks {
			projectTotal += sumEntries(t.Entries)
		}
		fmt.Fprintf(&b, "\n## %s (#%d) — %sh\n", p.Project.Name, p.Project.ID, trimFloat(projectTotal))
		for _, t := range p.TimeByTasks {
			fmt.Fprintf(&b, "- #%d %s — %sh", t.Activity.ID, t.Activity.Name, trimFloat(sumEntries(t.Entries)))
			if days := formatEntries(t.Entries); days != "" {
				fmt.Fprintf(&b, " (%s)", days)
			}
			b.WriteString("\n")
		}
	}

	if len(ts.TimeByNonProjectActivities) > 0 {
		b.WriteString("\n## Non-project activities\n")
		for _, a := range ts.TimeByNonProjectActivities {
			fmt.Fprintf(&b, "- %s (#%d) — %sh", a.Activity.Name, a.Activity.ID, trimFloat(sumEntries(a.Entries)))
			if days := formatEntries(a.Entries); days != "" {
				fmt.Fprintf(&b, " (%s)", days)
			}
			b.WriteString("\n")
		}
	}

	if days := formatEntries(ts.Total); days != "" {
		fmt.Fprintf(&b, "\n## Daily totals\n%s\n", days)
	}
	return b.String()
}

// FormatTimeReport renders the per-activity time report of a user.
func FormatTimeReport(tr *tp.TimeReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Time report — %s (#%d)\n", tr.User.Name, tr.User.ID)
	fmt.Fprintf(&b, "Period: %s → %s | Total: %sh\n", tr.FirstDay, tr.LastDay, trimFloat(sumEntries(tr.Total)))

	section := func(title string, lines []tp.ActivityLine) {
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n## %s\n", title)
		for _, l := range lines {
			fmt.Fprintf(&b, "- %s (#%d) — %sh", l.Activity.Name, l.Activity.ID, trimFloat(sumEntries(l.Entries)))
			if days := formatEntries(l.Entries); days != "" {
				fmt.Fprintf(&b, " (%s)", days)
			}
			b.WriteString("\n")
		}
	}
	section("Projects", tr.TimeByProjects)
	section("Non-project activities", tr.TimeByNonProjectActivities)

	if days := formatEntries(tr.Total); days != "" {
		fmt.Fprintf(&b, "\n## Daily totals\n%s\n", days)
	}
	return b.String()
}

// FormatAssignments renders a user's half-day assignment schedule.
func FormatAssignments(assignments []tp.Assignment, userID int) string {
	if len(assignments) == 0 {
		return fmt.Sprintf("No assignment for user #%d over this period.", userID)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Assignments for user #%d (%d half-days)\n\n", userID, len(assignments))
	for _, a := range assignments {
		fmt.Fprintf(&b, "- %s → %s\n", a.Date, entityLabel(a.To))
	}
	return b.String()
}

// --- Portfolios, risks, reference data ---

// FormatPortfolios renders a portfolio listing.
func FormatPortfolios(portfolios []tp.Portfolio) string {
	if len(portfolios) == 0 {
		return "No portfolio found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Portfolios (%d)\n\n", len(portfolios))
	for _, p := range portfolios {
		fmt.Fprintf(&b, "- #%d %s | %d project(s)", p.ID, p.Name, len(p.Projects))
		if p.Auto {
			b.WriteString(" | auto")
		}
		if p.Archived {
			b.WriteString(" | archived")
		}
		b.WriteString("\n")
		for _, pr := range p.Projects {
			fmt.Fprintf(&b, "    - #%d %s\n", pr.ID, pr.Name)
		}
	}
	return b.String()
}

// FormatRisks renders the risk register of a project.
func FormatRisks(risks []tp.Risk, projectID int) string {
	if len(risks) == 0 {
		return fmt.Sprintf("No risk registered on project #%d.", projectID)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Risks of project #%d (%d)\n\n", projectID, len(risks))
	for _, r := range risks {
		fmt.Fprintf(&b, "- #%d %s", r.ID, r.Name)
		var meta []string
		for label, v := range map[string]string{"state": r.State, "probability": r.Probability, "impact": r.Impact, "rating": r.Rating, "response": r.Response} {
			if v != "" {
				meta = append(meta, label+": "+v)
			}
		}
		sort.Strings(meta)
		if len(meta) > 0 {
			fmt.Fprintf(&b, " | %s", strings.Join(meta, " | "))
		}
		b.WriteString("\n")
		if r.Description != "" {
			fmt.Fprintf(&b, "    %s\n", strings.TrimSpace(r.Description))
		}
	}
	return b.String()
}

// FormatReference renders one of the reference lists.
func FormatReference(kind string, items []tp.ReferenceItem) string {
	if len(items) == 0 {
		return fmt.Sprintf("No entry in %s.", kind)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%d)\n\n", kind, len(items))
	var walk func(items []tp.ReferenceItem, depth int)
	walk = func(items []tp.ReferenceItem, depth int) {
		for _, it := range items {
			fmt.Fprintf(&b, "%s- #%d %s", strings.Repeat("  ", depth), it.ID, it.Name)
			if it.Archived {
				b.WriteString(" | archived")
			}
			if it.Unavailable {
				b.WriteString(" | unavailability")
			}
			if it.Finished {
				b.WriteString(" | end state")
			}
			if it.Description != "" {
				fmt.Fprintf(&b, " — %s", it.Description)
			}
			b.WriteString("\n")
			walk(it.Children, depth+1)
		}
	}
	walk(items, 0)
	return b.String()
}
