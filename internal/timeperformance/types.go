package timeperformance

// Entity is the short form every TimePerformance object is referenced by.
type Entity struct {
	ID         int    `json:"id"`
	Name       string `json:"name,omitempty"`
	Path       string `json:"path,omitempty"`
	Type       string `json:"type,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
	// Description is present on the "Element" flavour of the short form.
	Description string `json:"description,omitempty"`
}

// UnitValue is a number paired with its unit (currency code, HOURS, …).
type UnitValue struct {
	Val  float64 `json:"val"`
	Unit string  `json:"unit,omitempty"`
}

// ProjectState is a workflow state assignable to a project.
type ProjectState struct {
	Entity
	Finished bool `json:"finished,omitempty"`
}

// Project is a project or a project template.
type Project struct {
	Entity
	Priority     int           `json:"priority,omitempty"`
	Template     bool          `json:"template,omitempty"`
	ProjectType  string        `json:"projectType,omitempty"`
	Archived     bool          `json:"archived,omitempty"`
	Sponsor      string        `json:"sponsor,omitempty"`
	State        *ProjectState `json:"state,omitempty"`
	CreationDate string        `json:"creationDate,omitempty"`
	Labels       []Entity      `json:"labels,omitempty"`
}

// CheckListItem is one step of a task or risk check-list.
type CheckListItem struct {
	Name string `json:"name"`
	Done bool   `json:"done"`
}

// Task is a unit of work performed by a user on a project.
type Task struct {
	Entity
	Performer    *User           `json:"performer,omitempty"`
	Steps        []CheckListItem `json:"steps,omitempty"`
	DueDate      string          `json:"dueDate,omitempty"`
	Start        string          `json:"start,omitempty"`
	End          string          `json:"end,omitempty"`
	Effort       float64         `json:"effort,omitempty"`
	RTD          float64         `json:"rtd,omitempty"`
	State        string          `json:"state,omitempty"`
	Closed       bool            `json:"closed,omitempty"`
	Color        string          `json:"color,omitempty"`
	Project      *Entity         `json:"project,omitempty"`
	Phase        *Entity         `json:"phase,omitempty"`
	Iteration    *Entity         `json:"iteration,omitempty"`
	Goal         *Entity         `json:"goal,omitempty"`
	TaskType     *Entity         `json:"taskType,omitempty"`
	TaskCategory *Entity         `json:"taskCategory,omitempty"`
}

// CostRate converts worked hours into cost: an amount with its currency plus
// the mode it is applied in. Only visible with the right permissions.
type CostRate struct {
	UnitValue
	RateMode string `json:"rateMode,omitempty"`
}

// UserRights are the application-level permissions of a user.
type UserRights struct {
	AppAccess     bool `json:"appAccess,omitempty"`
	Administrator bool `json:"administrator,omitempty"`
	Manager       bool `json:"manager,omitempty"`
	PMO           bool `json:"pmo,omitempty"`
	LimitedAccess bool `json:"limitedAccess,omitempty"`
	CreateProject bool `json:"createProject,omitempty"`
	ManageSelf    bool `json:"manageSelf,omitempty"`
}

// User is an application user account.
type User struct {
	Entity
	Firstname         string      `json:"firstname,omitempty"`
	Lastname          string      `json:"lastname,omitempty"`
	Archived          bool        `json:"archived,omitempty"`
	Email             string      `json:"email,omitempty"`
	AvailabilityStart string      `json:"availabilityStart,omitempty"`
	AvailabilityEnd   string      `json:"availabilityEnd,omitempty"`
	Extern            bool        `json:"extern,omitempty"`
	Manager           bool        `json:"manager,omitempty"`
	Gender            string      `json:"gender,omitempty"`
	Locale            string      `json:"locale,omitempty"`
	OBS               *Entity     `json:"obs,omitempty"`
	Profile           *Entity     `json:"profile,omitempty"`
	CostRate          *CostRate   `json:"costRate,omitempty"`
	Rights            *UserRights `json:"rights,omitempty"`
}

// WBSElement is a deliverable or a phase — the API keeps the same shape for
// both, with children when the tree form is requested.
type WBSElement struct {
	Entity
	State          string       `json:"state,omitempty"`
	FirstDay       string       `json:"firstDay,omitempty"`
	LastDay        string       `json:"lastDay,omitempty"`
	Milestone      *bool        `json:"milestone,omitempty"`
	LockedSchedule *bool        `json:"lockedSchedule,omitempty"`
	ProgressMode   string       `json:"progressMode,omitempty"`
	Acceptance     string       `json:"acceptance,omitempty"`
	Iteration      *Entity      `json:"iteration,omitempty"`
	Phase          *Entity      `json:"phase,omitempty"`
	Children       []WBSElement `json:"children,omitempty"`
}

// Expense is a monetary expense recorded against a project.
type Expense struct {
	Entity
	Amount       *UnitValue `json:"amount,omitempty"`
	Date         string     `json:"date,omitempty"`
	Settled      bool       `json:"settled,omitempty"`
	Smoothed     bool       `json:"smoothed,omitempty"`
	Counterparty string     `json:"counterparty,omitempty"`
	Phase        *Entity    `json:"phase,omitempty"`
	Stage        *Entity    `json:"stage,omitempty"`
	Deliverable  *Entity    `json:"deliverable,omitempty"`
}

// Portfolio is a set of projects.
type Portfolio struct {
	Entity
	Auto     bool     `json:"auto,omitempty"`
	Archived bool     `json:"archived,omitempty"`
	Projects []Entity `json:"projects,omitempty"`
}

// TeamMember is a project team entry: the user, their skill profile and cost
// rate on that project, and their rights (MEMBER or PROJECT_MANAGER).
type TeamMember struct {
	User     User      `json:"user"`
	Profile  *Entity   `json:"profile,omitempty"`
	CostRate *CostRate `json:"costRate,omitempty"`
	Rights   string    `json:"rights,omitempty"`
}

// Risk is a project risk.
type Risk struct {
	Entity
	Probability     string          `json:"probability,omitempty"`
	Impact          string          `json:"impact,omitempty"`
	Rating          string          `json:"rating,omitempty"`
	Response        string          `json:"response,omitempty"`
	ResponseDetails string          `json:"responseDetails,omitempty"`
	State           string          `json:"state,omitempty"`
	CheckList       []CheckListItem `json:"checkList,omitempty"`
}

// DailyTimeEntry is the number of hours logged by a user on one day.
type DailyTimeEntry struct {
	Day   string  `json:"day"`
	Hours float64 `json:"hours"`
}

// Assignment is the assignment of a user to an activity for a half-day.
type Assignment struct {
	Date string  `json:"date"`
	User *Entity `json:"user,omitempty"`
	To   *Entity `json:"to,omitempty"`
}

// TimesheetTaskLine is the time logged on one task.
type TimesheetTaskLine struct {
	Activity Task             `json:"activity"`
	Entries  []DailyTimeEntry `json:"entries"`
}

// TimesheetProjectLine groups the tasks of one project.
type TimesheetProjectLine struct {
	Project     Entity              `json:"project"`
	TimeByTasks []TimesheetTaskLine `json:"timeByTasks"`
}

// ActivityLine is time logged against a project or a non-project activity,
// without the task breakdown.
type ActivityLine struct {
	Activity Entity           `json:"activity"`
	Entries  []DailyTimeEntry `json:"entries"`
}

// Timesheet is the per-task time report of a user over a period.
type Timesheet struct {
	User                       User                   `json:"user"`
	FirstDay                   string                 `json:"firstDay"`
	LastDay                    string                 `json:"lastDay"`
	TimeByProjects             []TimesheetProjectLine `json:"timeByProjects"`
	TimeByNonProjectActivities []ActivityLine         `json:"timeByNonProjectActivities"`
	Total                      []DailyTimeEntry       `json:"total"`
}

// TimeReport is the per-activity time report of a user over a period.
type TimeReport struct {
	User                       User             `json:"user"`
	FirstDay                   string           `json:"firstDay"`
	LastDay                    string           `json:"lastDay"`
	TimeByProjects             []ActivityLine   `json:"timeByProjects"`
	TimeByNonProjectActivities []ActivityLine   `json:"timeByNonProjectActivities"`
	Total                      []DailyTimeEntry `json:"total"`
}

// ReferenceItem is one entry of a reference list (OBS, skill profiles,
// non-project activities, project states, custom forms).
type ReferenceItem struct {
	Entity
	Archived    bool            `json:"archived,omitempty"`
	Unavailable bool            `json:"unavailable,omitempty"`
	Finished    bool            `json:"finished,omitempty"`
	Children    []ReferenceItem `json:"children,omitempty"`
}

// --- Write payloads ---

// ProjectWrite carries the writable fields of a project. Pointer fields are
// omitted from the JSON payload when nil, so a PATCH only touches what was set.
type ProjectWrite struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	ExternalID     *string `json:"externalId,omitempty"`
	Priority       *int    `json:"priority,omitempty"`
	ProjectType    *string `json:"projectType,omitempty"`
	Sponsor        *string `json:"sponsor,omitempty"`
	State          *int    `json:"state,omitempty"`
	Labels         []int   `json:"labels,omitempty"`
	ProjectManager *int    `json:"projectManager,omitempty"`
	PhasingDepth   *int    `json:"phasingDepth,omitempty"`
	Template       *int    `json:"template,omitempty"`
}

// TaskWrite carries the writable fields of a task.
type TaskWrite struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	ExternalID  *string `json:"externalId,omitempty"`
	DueDate     *string `json:"dueDate,omitempty"`
	Performer   *int    `json:"performer,omitempty"`
	Goal        *int    `json:"goal,omitempty"`
	TaskType    *int    `json:"taskType,omitempty"`
}

// WBSWrite carries the writable fields of a deliverable or a phase.
// Acceptance, ProgressMode and Iteration only apply to deliverables.
type WBSWrite struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	ExternalID     *string `json:"externalId,omitempty"`
	State          *string `json:"state,omitempty"`
	FirstDay       *string `json:"firstDay,omitempty"`
	LastDay        *string `json:"lastDay,omitempty"`
	Milestone      *bool   `json:"milestone,omitempty"`
	LockedSchedule *bool   `json:"lockedSchedule,omitempty"`
	ProgressMode   *string `json:"progressMode,omitempty"`
	Acceptance     *string `json:"acceptance,omitempty"`
	Iteration      *int    `json:"iteration,omitempty"`
}

// ExpenseWrite carries the writable fields of an expense.
type ExpenseWrite struct {
	Name         *string    `json:"name,omitempty"`
	Description  *string    `json:"description,omitempty"`
	ExternalID   *string    `json:"externalId,omitempty"`
	Amount       *UnitValue `json:"amount,omitempty"`
	Date         *string    `json:"date,omitempty"`
	Counterparty *string    `json:"counterparty,omitempty"`
	Smoothed     *bool      `json:"smoothed,omitempty"`
	Deliverable  *int       `json:"deliverable,omitempty"`
}

// UserRightsWrite carries the writable rights of a user.
type UserRightsWrite struct {
	AppAccess     *bool `json:"appAccess,omitempty"`
	Administrator *bool `json:"administrator,omitempty"`
	Manager       *bool `json:"manager,omitempty"`
	PMO           *bool `json:"pmo,omitempty"`
	LimitedAccess *bool `json:"limitedAccess,omitempty"`
	CreateProject *bool `json:"createProject,omitempty"`
	ManageSelf    *bool `json:"manageSelf,omitempty"`
}

// UserWrite carries the writable fields of a user account.
type UserWrite struct {
	Firstname         *string          `json:"firstname,omitempty"`
	Lastname          *string          `json:"lastname,omitempty"`
	Email             *string          `json:"email,omitempty"`
	ExternalID        *string          `json:"externalId,omitempty"`
	AvailabilityStart *string          `json:"availabilityStart,omitempty"`
	AvailabilityEnd   *string          `json:"availabilityEnd,omitempty"`
	OBS               *int             `json:"obs,omitempty"`
	Profile           *int             `json:"profile,omitempty"`
	Gender            *string          `json:"gender,omitempty"`
	Locale            *string          `json:"locale,omitempty"`
	SSOUID            *string          `json:"ssoUID,omitempty"`
	Extern            *bool            `json:"extern,omitempty"`
	Rights            *UserRightsWrite `json:"rights,omitempty"`
}
