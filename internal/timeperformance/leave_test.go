package timeperformance

import (
	"reflect"
	"testing"
)

func TestHalfDays(t *testing.T) {
	// 2026-10-02 is a Friday.
	got, err := HalfDays("2026-10-02", "PM", "2026-10-05", "AM", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-10-02 PM", "2026-10-05 AM"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	got, _ = HalfDays("2026-10-03", "", "2026-10-03", "", true)
	if want := []string{"2026-10-03 AM", "2026-10-03 PM"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("weekends: got %v, want %v", got, want)
	}

	for _, c := range [][4]string{
		{"2026-10-03", "", "2026-10-04", ""},     // weekend only
		{"2026-10-05", "PM", "2026-10-05", "AM"}, // ends before it starts
		{"2026-10-06", "", "2026-10-05", ""},
		{"05/10/2026", "", "2026-10-05", ""},
		{"2026-10-05", "XX", "2026-10-05", ""},
	} {
		if _, err := HalfDays(c[0], c[1], c[2], c[3], false); err == nil {
			t.Errorf("%v: expected an error", c)
		}
	}
}

func assign(date, typ string, id int) Assignment {
	return Assignment{Date: date, To: &Entity{ID: id, Type: typ}}
}

func TestPlanLeaveAddKeepsOtherUnavailabilities(t *testing.T) {
	unavailable := map[int]bool{21: true, 22: true, 99: true}
	existing := []Assignment{
		assign("2026-10-05 AM", "npactivity", 22), // other leave, inside the window, outside the request
		assign("2026-10-05 PM", "project", 7),     // project work: not ours to sync
		assign("2026-10-06 AM", "npactivity", 22), // replaced by the new leave
		assign("2026-10-06 PM", "npactivity", 21), // already there
		assign("2026-10-07 AM", "npactivity", 99), // archived type, must survive
		assign("2026-10-07 PM", "npactivity", 5),  // non-unavailability activity
		assign("2026-10-09 AM", "npactivity", 22), // outside the window
	}
	days := []string{"2026-10-05 PM", "2026-10-06 AM", "2026-10-06 PM"}
	plan := PlanLeave(existing, unavailable, days, 21, false, SyncAllOrNothing)

	want := UnavailabilitySync{
		SyncMode: SyncAllOrNothing,
		FirstDay: "2026-10-05",
		LastDay:  "2026-10-06",
		Assignments: []UnavailabilityAssignment{
			{ActivityID: 21, Periods: []string{"2026-10-05 PM", "2026-10-06 AM", "2026-10-06 PM"}},
			{ActivityID: 22, Periods: []string{"2026-10-05 AM"}},
		},
	}
	if !reflect.DeepEqual(plan.Sync, want) {
		t.Fatalf("sync:\n got %+v\nwant %+v", plan.Sync, want)
	}
	if !reflect.DeepEqual(plan.Added, []string{"2026-10-05 PM", "2026-10-06 AM"}) {
		t.Errorf("added = %v", plan.Added)
	}
	if !reflect.DeepEqual(plan.Removed, []string{"2026-10-06 AM → #22"}) {
		t.Errorf("removed = %v", plan.Removed)
	}
	if plan.Kept != 1 {
		t.Errorf("kept = %d, want 1", plan.Kept)
	}
}

func TestPlanLeaveKeepsArchivedTypeInWindow(t *testing.T) {
	unavailable := map[int]bool{21: true, 99: true}
	existing := []Assignment{assign("2026-10-05 AM", "npactivity", 99)}
	plan := PlanLeave(existing, unavailable, []string{"2026-10-05 PM"}, 21, false, SyncForceOverride)
	if len(plan.Sync.Assignments) != 2 || plan.Sync.Assignments[1].ActivityID != 99 {
		t.Fatalf("archived unavailability dropped: %+v", plan.Sync.Assignments)
	}
}

func TestPlanLeaveRemove(t *testing.T) {
	unavailable := map[int]bool{21: true, 22: true}
	existing := []Assignment{
		assign("2026-10-05 AM", "npactivity", 21),
		assign("2026-10-05 PM", "npactivity", 22),
	}
	days := []string{"2026-10-05 AM", "2026-10-05 PM"}

	plan := PlanLeave(existing, unavailable, days, 21, true, SyncAllOrNothing)
	if !reflect.DeepEqual(plan.Sync.Assignments, []UnavailabilityAssignment{{ActivityID: 22, Periods: []string{"2026-10-05 PM"}}}) {
		t.Errorf("typed remove: %+v", plan.Sync.Assignments)
	}

	plan = PlanLeave(existing, unavailable, days, 0, true, SyncAllOrNothing)
	if len(plan.Sync.Assignments) != 0 || len(plan.Removed) != 2 {
		t.Errorf("remove all: %+v removed=%v", plan.Sync.Assignments, plan.Removed)
	}
	if plan.Sync.Assignments == nil {
		t.Error("assignments must encode as [] not null")
	}
}

func TestResolveUnavailability(t *testing.T) {
	items := []ReferenceItem{
		{Entity: Entity{ID: 21, Name: "Congés payés"}, Unavailable: true},
		{Entity: Entity{ID: 22, Name: "Congé maladie"}, Unavailable: true},
		{Entity: Entity{ID: 99, Name: "RTT"}, Unavailable: true, Archived: true},
	}
	if it, err := ResolveUnavailability(items, "congés payés"); err != nil || it.ID != 21 {
		t.Errorf("exact: %v %v", it, err)
	}
	if it, err := ResolveUnavailability(items, "maladie"); err != nil || it.ID != 22 {
		t.Errorf("substring: %v %v", it, err)
	}
	if _, err := ResolveUnavailability(items, "congé"); err == nil {
		t.Error("ambiguous name should fail")
	}
	if _, err := ResolveUnavailability(items, "99"); err == nil {
		t.Error("archived type should not resolve")
	}
}
