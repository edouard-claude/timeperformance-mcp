package timeperformance

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Sync modes of POST /users/{id}/assignments/syncUnavailabilities.
const (
	SyncAllOrNothing    = "ALL_OR_NOTHING"
	SyncNonConflictOnly = "NON_CONFLICT_ONLY"
	SyncForceOverride   = "FORCE_OVERRIDE"
)

// SyncModes lists the accepted sync modes.
var SyncModes = []string{SyncAllOrNothing, SyncNonConflictOnly, SyncForceOverride}

// UnavailabilityAssignment is the set of half-days planned on one unavailability.
type UnavailabilityAssignment struct {
	ActivityID int      `json:"activityId"`
	Periods    []string `json:"periods"`
}

// UnavailabilitySync is the body of syncUnavailabilities. The API treats it as
// the whole truth over [FirstDay, LastDay]: any unavailability of the person in
// that window that is not listed is removed.
type UnavailabilitySync struct {
	SyncMode    string                     `json:"syncMode"`
	FirstDay    string                     `json:"firstDay"`
	LastDay     string                     `json:"lastDay"`
	Assignments []UnavailabilityAssignment `json:"assignments"`
}

// SyncResult is the answer of syncUnavailabilities. Success is false only when
// there are conflicts and the mode is ALL_OR_NOTHING.
type SyncResult struct {
	Success   bool     `json:"success"`
	Updated   int      `json:"updated"`
	Conflicts []string `json:"conflicts"`
}

// SyncUserUnavailabilities replaces a user's unavailabilities over a period.
func (c *Client) SyncUserUnavailabilities(userID int, s UnavailabilitySync) (*SyncResult, error) {
	var out SyncResult
	if err := c.send(http.MethodPost, fmt.Sprintf("/users/%d/assignments/syncUnavailabilities", userID), nil, s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListUnavailabilities returns the non-project activities flagged as
// unavailabilities (leaves, holidays…), archived ones included: a sync must
// keep what is already planned on them.
func (c *Client) ListUnavailabilities() ([]ReferenceItem, error) {
	items, err := c.ListReference("npactivities")
	if err != nil {
		return nil, err
	}
	var out []ReferenceItem
	for _, it := range items {
		if it.Unavailable {
			out = append(out, it)
		}
	}
	return out, nil
}

// ResolveUnavailability accepts a numeric id or a name (exact, then unique
// substring, case-insensitive) among the non-archived unavailabilities.
func ResolveUnavailability(all []ReferenceItem, ref string) (ReferenceItem, error) {
	var items []ReferenceItem
	for _, it := range all {
		if !it.Archived {
			items = append(items, it)
		}
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ReferenceItem{}, fmt.Errorf("a leave type (unavailability id or name) is required")
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, fmt.Sprintf("#%d %s", it.ID, it.Name))
	}
	available := strings.Join(names, ", ")

	if id, err := strconv.Atoi(ref); err == nil {
		for _, it := range items {
			if it.ID == id {
				return it, nil
			}
		}
		return ReferenceItem{}, fmt.Errorf("#%d is not an active unavailability (available: %s)", id, available)
	}
	var matches []ReferenceItem
	for _, it := range items {
		if strings.EqualFold(it.Name, ref) {
			return it, nil
		}
		if strings.Contains(strings.ToLower(it.Name), strings.ToLower(ref)) {
			matches = append(matches, it)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return ReferenceItem{}, fmt.Errorf("no unavailability matches %q (available: %s)", ref, available)
	default:
		return ReferenceItem{}, fmt.Errorf("%q matches several unavailabilities (available: %s) — pass the id", ref, available)
	}
}

// HalfDays lists the half-days from firstDay/firstHalf to lastDay/lastHalf,
// inclusive, formatted "YYYY-MM-DD AM|PM". Halves default to AM and PM.
// Saturdays and Sundays are skipped unless weekends is set.
func HalfDays(firstDay, firstHalf, lastDay, lastHalf string, weekends bool) ([]string, error) {
	first, err := time.Parse(time.DateOnly, strings.TrimSpace(firstDay))
	if err != nil {
		return nil, fmt.Errorf("first day %q is not an ISO 8601 date", firstDay)
	}
	last, err := time.Parse(time.DateOnly, strings.TrimSpace(lastDay))
	if err != nil {
		return nil, fmt.Errorf("last day %q is not an ISO 8601 date", lastDay)
	}
	fh, err := parseHalf(firstHalf, "AM")
	if err != nil {
		return nil, err
	}
	lh, err := parseHalf(lastHalf, "PM")
	if err != nil {
		return nil, err
	}
	if last.Before(first) || (last.Equal(first) && fh == "PM" && lh == "AM") {
		return nil, fmt.Errorf("the period ends before it starts")
	}

	var out []string
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		if !weekends && (d.Weekday() == time.Saturday || d.Weekday() == time.Sunday) {
			continue
		}
		day := d.Format(time.DateOnly)
		if !(d.Equal(first) && fh == "PM") {
			out = append(out, day+" AM")
		}
		if !(d.Equal(last) && lh == "AM") {
			out = append(out, day+" PM")
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the period only covers a weekend")
	}
	return out, nil
}

func parseHalf(s, def string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "":
		return def, nil
	case "AM", "PM":
		return s, nil
	}
	return "", fmt.Errorf("half %q must be AM or PM", s)
}

// LeavePlan is the outcome of merging a leave change into a user's current
// unavailabilities: the full sync payload plus what actually changes.
type LeavePlan struct {
	Sync    UnavailabilitySync
	Added   []string // half-days newly planned on the requested activity
	Removed []string // half-days taken off an unavailability (as "day half → activity id")
	Kept    int      // half-days of other unavailabilities left untouched in the window
}

// PlanLeave builds the syncUnavailabilities payload that adds (or, with remove,
// clears) days on activityID while keeping every other unavailability found in
// existing. existing must cover at least the days of the window; unavailable
// says which npactivity ids are unavailabilities. On remove, activityID 0
// clears any unavailability on those half-days.
func PlanLeave(existing []Assignment, unavailable map[int]bool, days []string, activityID int, remove bool, mode string) LeavePlan {
	firstDay, lastDay := days[0][:10], days[len(days)-1][:10]
	current := map[string]int{} // half-day → unavailability id
	for _, a := range existing {
		if a.To == nil || a.To.Type != "npactivity" || !unavailable[a.To.ID] {
			continue
		}
		if d := a.Date; len(d) >= 10 && d[:10] >= firstDay && d[:10] <= lastDay {
			current[d] = a.To.ID
		}
	}

	plan := LeavePlan{}
	target := map[string]bool{}
	for _, d := range days {
		target[d] = true
		prev, had := current[d]
		switch {
		case remove:
			if had && (activityID == 0 || prev == activityID) {
				plan.Removed = append(plan.Removed, fmt.Sprintf("%s → #%d", d, prev))
				delete(current, d)
			}
		case !had:
			plan.Added = append(plan.Added, d)
			current[d] = activityID
		case prev != activityID:
			plan.Removed = append(plan.Removed, fmt.Sprintf("%s → #%d", d, prev))
			plan.Added = append(plan.Added, d)
			current[d] = activityID
		}
	}

	byActivity := map[int][]string{}
	for d, id := range current {
		byActivity[id] = append(byActivity[id], d)
		if !target[d] {
			plan.Kept++
		}
	}
	ids := make([]int, 0, len(byActivity))
	for id := range byActivity {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	assignments := make([]UnavailabilityAssignment, 0, len(ids))
	for _, id := range ids {
		periods := byActivity[id]
		sort.Strings(periods)
		assignments = append(assignments, UnavailabilityAssignment{ActivityID: id, Periods: periods})
	}
	plan.Sync = UnavailabilitySync{SyncMode: mode, FirstDay: firstDay, LastDay: lastDay, Assignments: assignments}
	return plan
}

// LeaveRequest describes a leave to add or remove for one user.
type LeaveRequest struct {
	UserID    int
	Activity  string // unavailability id or name; optional on remove
	FirstDay  string
	FirstHalf string // AM (default) or PM
	LastDay   string
	LastHalf  string // PM (default) or AM
	Weekends  bool
	Remove    bool
	SyncMode  string // default ALL_OR_NOTHING
}

// PlanUserLeave reads the user's current schedule and returns the merged
// sync payload for req, along with the resolved unavailability (zero value
// when removing without an activity).
func (c *Client) PlanUserLeave(req LeaveRequest) (LeavePlan, ReferenceItem, error) {
	mode := strings.ToUpper(strings.TrimSpace(req.SyncMode))
	if mode == "" {
		mode = SyncAllOrNothing
	}
	if !slices.Contains(SyncModes, mode) {
		return LeavePlan{}, ReferenceItem{}, fmt.Errorf("sync mode %q must be one of %s", req.SyncMode, strings.Join(SyncModes, ", "))
	}
	days, err := HalfDays(req.FirstDay, req.FirstHalf, req.LastDay, req.LastHalf, req.Weekends)
	if err != nil {
		return LeavePlan{}, ReferenceItem{}, err
	}
	items, err := c.ListUnavailabilities()
	if err != nil {
		return LeavePlan{}, ReferenceItem{}, fmt.Errorf("list unavailabilities: %w", err)
	}
	var activity ReferenceItem
	if !req.Remove || strings.TrimSpace(req.Activity) != "" {
		if activity, err = ResolveUnavailability(items, req.Activity); err != nil {
			return LeavePlan{}, ReferenceItem{}, err
		}
	}
	unavailable := map[int]bool{}
	for _, it := range items {
		unavailable[it.ID] = true
	}
	existing, err := c.GetUserAssignments(req.UserID, days[0][:10], days[len(days)-1][:10])
	if err != nil {
		return LeavePlan{}, ReferenceItem{}, fmt.Errorf("read current schedule: %w", err)
	}
	return PlanLeave(existing, unavailable, days, activity.ID, req.Remove, mode), activity, nil
}
