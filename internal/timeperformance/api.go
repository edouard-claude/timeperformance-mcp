package timeperformance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// --- Projects ---

// ListProjects returns every project the credentials can see.
func (c *Client) ListProjects(includeArchived bool) ([]Project, error) {
	params := url.Values{}
	if includeArchived {
		params.Set("archived", "true")
	}
	var out []Project
	if err := c.get("/projects", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListProjectTemplates returns the project templates.
func (c *Client) ListProjectTemplates(includeArchived bool) ([]Project, error) {
	params := url.Values{}
	if includeArchived {
		params.Set("archived", "true")
	}
	var out []Project
	if err := c.get("/projects/templates", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetProject returns a single project.
func (c *Client) GetProject(id int) (*Project, error) {
	var out Project
	if err := c.get(fmt.Sprintf("/projects/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateProject creates a project. The back-office API requires projectManager.
func (c *Client) CreateProject(p ProjectWrite) (*Project, error) {
	var out Project
	if err := c.send(http.MethodPost, "/projects", nil, p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateProject patches the writable fields of a project.
func (c *Client) UpdateProject(id int, p ProjectWrite) (*Project, error) {
	var out Project
	if err := c.send(http.MethodPatch, fmt.Sprintf("/projects/%d", id), nil, p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProjectReportPath maps a report kind to its endpoint suffix. The reports are
// deep, report-specific trees, returned as raw JSON rather than mirrored types.
var ProjectReportPath = map[string]string{
	"progress":     "/progressReport",
	"roadmap":      "/roadmap",
	"brief":        "/brief",
	"baseline":     "/baselineReport",
	"config":       "/config",
	"workload":     "/workloadplan",
	"actuals":      "/actualsTimeseries",
	"assignments":  "/assignments",
	"datasheets":   "/datasheets",
	"lastmodified": "/lastmodified",
}

// ProjectReport fetches one of the project report endpoints as raw JSON.
func (c *Client) ProjectReport(projectID int, kind string, params url.Values) (json.RawMessage, error) {
	suffix, ok := ProjectReportPath[kind]
	if !ok {
		return nil, fmt.Errorf("unknown report kind %q", kind)
	}
	return c.GetRaw(fmt.Sprintf("/projects/%d%s", projectID, suffix), params)
}

// ListProjectRisks returns the risks registered on a project.
func (c *Client) ListProjectRisks(projectID int) ([]Risk, error) {
	var out []Risk
	if err := c.get(fmt.Sprintf("/projects/%d/risks", projectID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- Tasks ---

// TaskFilter narrows a task listing.
type TaskFilter struct {
	NotClosed   bool
	PeriodStart string
	PeriodEnd   string
}

func (f TaskFilter) values() url.Values {
	params := url.Values{}
	if f.NotClosed {
		params.Set("notClosed", "true")
	}
	if f.PeriodStart != "" {
		params.Set("periodStart", f.PeriodStart)
	}
	if f.PeriodEnd != "" {
		params.Set("periodEnd", f.PeriodEnd)
	}
	return params
}

// ListProjectTasks returns the tasks of a project.
func (c *Client) ListProjectTasks(projectID int, f TaskFilter) ([]Task, error) {
	var out []Task
	if err := c.get(fmt.Sprintf("/projects/%d/tasks", projectID), f.values(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListUserTasks returns the tasks assigned to a user, across projects.
func (c *Client) ListUserTasks(userID int, f TaskFilter) ([]Task, error) {
	var out []Task
	if err := c.get(fmt.Sprintf("/users/%d/tasks", userID), f.values(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetUserTodoList returns the current to-do list of a user, optionally scoped
// to one project.
func (c *Client) GetUserTodoList(userID int, projectID int) ([]Task, error) {
	params := url.Values{}
	if projectID > 0 {
		params.Set("project", strconv.Itoa(projectID))
	}
	var out []Task
	if err := c.get(fmt.Sprintf("/users/%d/todolist", userID), params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTask returns a single task.
func (c *Client) GetTask(taskID int) (*Task, error) {
	var out Task
	if err := c.get(fmt.Sprintf("/projects/tasks/%d", taskID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateTask adds a task to a project.
func (c *Client) CreateTask(projectID int, t TaskWrite) (*Task, error) {
	var out Task
	if err := c.send(http.MethodPost, fmt.Sprintf("/projects/%d/tasks", projectID), nil, t, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTask patches the writable fields of a task.
func (c *Client) UpdateTask(taskID int, t TaskWrite) (*Task, error) {
	var out Task
	if err := c.send(http.MethodPatch, fmt.Sprintf("/projects/tasks/%d", taskID), nil, t, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Deliverables & phases ---

// ListDeliverables returns the deliverables tree of a project.
func (c *Client) ListDeliverables(projectID int) ([]WBSElement, error) {
	var out []WBSElement
	if err := c.get(fmt.Sprintf("/projects/%d/deliverables", projectID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetDeliverable returns a single deliverable.
func (c *Client) GetDeliverable(id int) (*WBSElement, error) {
	var out WBSElement
	if err := c.get(fmt.Sprintf("/projects/deliverables/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateDeliverable adds a top-level deliverable to a project.
func (c *Client) CreateDeliverable(projectID int, w WBSWrite) (*WBSElement, error) {
	var out WBSElement
	if err := c.send(http.MethodPost, fmt.Sprintf("/projects/%d/deliverables", projectID), nil, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateSubDeliverable adds a deliverable under an existing one.
func (c *Client) CreateSubDeliverable(parentID int, w WBSWrite) (*WBSElement, error) {
	var out WBSElement
	if err := c.send(http.MethodPost, fmt.Sprintf("/projects/deliverables/%d/subdeliverables", parentID), nil, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDeliverable patches a deliverable.
func (c *Client) UpdateDeliverable(id int, w WBSWrite) (*WBSElement, error) {
	var out WBSElement
	if err := c.send(http.MethodPatch, fmt.Sprintf("/projects/deliverables/%d", id), nil, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListPhases returns the phases tree of a project.
func (c *Client) ListPhases(projectID int) ([]WBSElement, error) {
	var out []WBSElement
	if err := c.get(fmt.Sprintf("/projects/%d/phases", projectID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPhase returns a single phase.
func (c *Client) GetPhase(id int) (*WBSElement, error) {
	var out WBSElement
	if err := c.get(fmt.Sprintf("/projects/phases/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreatePhase adds a top-level phase to a project.
func (c *Client) CreatePhase(projectID int, w WBSWrite) (*WBSElement, error) {
	var out WBSElement
	if err := c.send(http.MethodPost, fmt.Sprintf("/projects/%d/phases", projectID), nil, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateSubPhase adds a phase under an existing one.
func (c *Client) CreateSubPhase(parentID int, w WBSWrite) (*WBSElement, error) {
	var out WBSElement
	if err := c.send(http.MethodPost, fmt.Sprintf("/projects/phases/%d/subphases", parentID), nil, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdatePhase patches a phase.
func (c *Client) UpdatePhase(id int, w WBSWrite) (*WBSElement, error) {
	var out WBSElement
	if err := c.send(http.MethodPatch, fmt.Sprintf("/projects/phases/%d", id), nil, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Expenses ---

// ListExpenses returns the expenses of a project.
func (c *Client) ListExpenses(projectID int) ([]Expense, error) {
	var out []Expense
	if err := c.get(fmt.Sprintf("/projects/%d/expenses", projectID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetExpense returns a single expense.
func (c *Client) GetExpense(id int) (*Expense, error) {
	var out Expense
	if err := c.get(fmt.Sprintf("/projects/expenses/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateExpense records an expense on a project.
func (c *Client) CreateExpense(projectID int, w ExpenseWrite) (*Expense, error) {
	var out Expense
	if err := c.send(http.MethodPost, fmt.Sprintf("/projects/%d/expenses", projectID), nil, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateExpense patches an expense.
func (c *Client) UpdateExpense(id int, w ExpenseWrite) (*Expense, error) {
	var out Expense
	if err := c.send(http.MethodPatch, fmt.Sprintf("/projects/expenses/%d", id), nil, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Deletion ---

// DeletePath maps a deletable element kind to its endpoint.
var DeletePath = map[string]string{
	"task":        "/projects/tasks/%d",
	"deliverable": "/projects/deliverables/%d",
	"phase":       "/projects/phases/%d",
	"expense":     "/projects/expenses/%d",
	"datasheet":   "/projects/datasheets/%d",
}

// DeleteElement permanently deletes a task, deliverable, phase, expense or
// datasheet.
func (c *Client) DeleteElement(kind string, id int) error {
	pattern, ok := DeletePath[kind]
	if !ok {
		return fmt.Errorf("unknown element kind %q", kind)
	}
	return c.send(http.MethodDelete, fmt.Sprintf(pattern, id), nil, nil, nil)
}

// --- Project team ---

// ListTeam returns the members of a project with their rights.
func (c *Client) ListTeam(projectID int) ([]TeamMember, error) {
	var out []TeamMember
	if err := c.get(fmt.Sprintf("/projects/%d/team", projectID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddTeamMember adds a user to a project team. profileID is optional (0 skips).
func (c *Client) AddTeamMember(projectID, userID, profileID int, projectManager bool) error {
	params := url.Values{"user": {strconv.Itoa(userID)}}
	if profileID > 0 {
		params.Set("profile", strconv.Itoa(profileID))
	}
	if projectManager {
		params.Set("projectManager", "true")
	}
	return c.send(http.MethodPost, fmt.Sprintf("/projects/%d/team/addMember", projectID), params, nil, nil)
}

// SetTeamMemberRights toggles the project-manager right of a team member.
func (c *Client) SetTeamMemberRights(projectID, userID int, projectManager bool) error {
	params := url.Values{
		"user":           {strconv.Itoa(userID)},
		"projectManager": {strconv.FormatBool(projectManager)},
	}
	return c.send(http.MethodPost, fmt.Sprintf("/projects/%d/team/setMemberRights", projectID), params, nil, nil)
}

// RemoveTeamMember removes a user from a project team.
func (c *Client) RemoveTeamMember(projectID, userID int) error {
	params := url.Values{"user": {strconv.Itoa(userID)}}
	return c.send(http.MethodPost, fmt.Sprintf("/projects/%d/team/removeMember", projectID), params, nil, nil)
}

// --- Users ---

// ListUsers returns the user directory.
func (c *Client) ListUsers() ([]User, error) {
	var out []User
	if err := c.get("/users", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetUser returns a single user.
func (c *Client) GetUser(id int) (*User, error) {
	var out User
	if err := c.get(fmt.Sprintf("/users/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateUser creates a user account.
func (c *Client) CreateUser(u UserWrite) (*User, error) {
	var out User
	if err := c.send(http.MethodPost, "/users", nil, u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateUser patches a user account.
func (c *Client) UpdateUser(id int, u UserWrite) (*User, error) {
	var out User
	if err := c.send(http.MethodPatch, fmt.Sprintf("/users/%d", id), nil, u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveUser archives a user account (reversible in the web app).
func (c *Client) ArchiveUser(id int) error {
	return c.send(http.MethodPost, fmt.Sprintf("/users/%d/archive", id), nil, nil, nil)
}

// DeleteUser permanently deletes a user account.
func (c *Client) DeleteUser(id int) error {
	return c.send(http.MethodDelete, fmt.Sprintf("/users/%d", id), nil, nil, nil)
}

// GetUserAssignments returns the assignment schedule of a user over a period.
func (c *Client) GetUserAssignments(userID int, firstDay, lastDay string) ([]Assignment, error) {
	params := url.Values{"firstDay": {firstDay}, "lastDay": {lastDay}}
	var out []Assignment
	if err := c.get(fmt.Sprintf("/users/%d/assignments", userID), params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// TimeParams are the shared options of the timesheet and time report endpoints.
type TimeParams struct {
	FirstDay         string
	LastDay          string
	HoursPerDay      float64
	HalfDayThreshold float64
	SortTasks        bool
}

func (p TimeParams) values() url.Values {
	params := url.Values{"firstDay": {p.FirstDay}, "lastDay": {p.LastDay}}
	if p.HoursPerDay > 0 {
		params.Set("hoursPerDay", strconv.FormatFloat(p.HoursPerDay, 'f', -1, 64))
	}
	if p.HalfDayThreshold > 0 {
		params.Set("halfDayThreshold", strconv.FormatFloat(p.HalfDayThreshold, 'f', -1, 64))
	}
	if p.SortTasks {
		params.Set("sortTasks", "true")
	}
	return params
}

// GetUserTimesheet returns the per-task time entries of a user over a period.
func (c *Client) GetUserTimesheet(userID int, p TimeParams) (*Timesheet, error) {
	var out Timesheet
	if err := c.get(fmt.Sprintf("/users/%d/timesheet", userID), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetUserTimeReport returns the per-activity time entries of a user over a period.
func (c *Client) GetUserTimeReport(userID int, p TimeParams) (*TimeReport, error) {
	var out TimeReport
	if err := c.get(fmt.Sprintf("/users/%d/timeReport", userID), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Portfolios ---

// ListPortfolios returns the portfolios.
func (c *Client) ListPortfolios(includeArchived bool) ([]Portfolio, error) {
	params := url.Values{}
	if includeArchived {
		params.Set("archived", "true")
	}
	var out []Portfolio
	if err := c.get("/portfolios", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PortfolioProgressReport returns the progress report of a portfolio as raw JSON.
func (c *Client) PortfolioProgressReport(id int) (json.RawMessage, error) {
	return c.GetRaw(fmt.Sprintf("/portfolios/%d/progressReport", id), nil)
}

// --- Reference data ---

// ReferencePath maps a reference list kind to its endpoint.
var ReferencePath = map[string]string{
	"obs":           "/obs",
	"profiles":      "/profiles",
	"npactivities":  "/npactivities",
	"projectstates": "/projectstates",
	"customforms":   "/customforms",
}

// ListReference returns one of the reference lists.
func (c *Client) ListReference(kind string) ([]ReferenceItem, error) {
	path, ok := ReferencePath[kind]
	if !ok {
		return nil, fmt.Errorf("unknown reference kind %q", kind)
	}
	var out []ReferenceItem
	if err := c.get(path, nil, &out); err != nil {
		// /obs may return a single root node rather than a list.
		var single ReferenceItem
		if raw, rawErr := c.GetRaw(path, nil); rawErr == nil {
			if jsonErr := json.Unmarshal(raw, &single); jsonErr == nil && single.ID != 0 {
				return []ReferenceItem{single}, nil
			}
		}
		return nil, err
	}
	return out, nil
}
