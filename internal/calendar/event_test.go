package calendar

import (
	"reflect"
	"testing"
	"time"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestMergeSortsByStart(t *testing.T) {
	a := []Event{{UID: "a", Title: "Late", Start: at("2026-06-10T15:00:00Z"), End: at("2026-06-10T16:00:00Z"), Markers: []string{"A"}}}
	b := []Event{{UID: "b", Title: "Early", Start: at("2026-06-10T09:00:00Z"), End: at("2026-06-10T10:00:00Z"), Markers: []string{"B"}}}

	got := Merge(a, b)
	if len(got) != 2 || got[0].Title != "Early" || got[1].Title != "Late" {
		t.Fatalf("not sorted by start: %+v", got)
	}
}

func TestMergeDedupSharedUID(t *testing.T) {
	start := at("2026-06-10T18:00:00Z")
	end := at("2026-06-10T20:00:00Z")
	// Same invite on Mom's and Dad's calendars: same iCalUID + start.
	mom := []Event{{UID: "shared@google.com", Title: "Dinner", Start: start, End: end, AccountID: 1, Markers: []string{"M"}}}
	dad := []Event{{UID: "shared@google.com", Title: "Dinner", Start: start, End: end, AccountID: 2, Markers: []string{"D"}}}

	got := Merge(mom, dad)
	if len(got) != 1 {
		t.Fatalf("shared event not deduplicated: %+v", got)
	}
	if !reflect.DeepEqual(got[0].Markers, []string{"M", "D"}) {
		t.Errorf("markers not accumulated: %v", got[0].Markers)
	}
	// Surviving event keeps the first-seen account.
	if got[0].AccountID != 1 {
		t.Errorf("AccountID = %d, want 1", got[0].AccountID)
	}
}

func TestMergeRecurrenceInstancesKept(t *testing.T) {
	// Same series UID, different start times -> distinct instances, both kept.
	evs := []Event{
		{UID: "series@google.com", Title: "Standup", Start: at("2026-06-10T09:00:00Z"), End: at("2026-06-10T09:15:00Z"), Markers: []string{"A"}},
		{UID: "series@google.com", Title: "Standup", Start: at("2026-06-11T09:00:00Z"), End: at("2026-06-11T09:15:00Z"), Markers: []string{"A"}},
	}
	if got := Merge(evs); len(got) != 2 {
		t.Fatalf("recurrence instances collapsed: %+v", got)
	}
}

func TestMergeFuzzyFallbackNoUID(t *testing.T) {
	start := at("2026-06-10T12:00:00Z")
	end := at("2026-06-10T13:00:00Z")
	// No UID: dedup falls back to normalized title + start + end.
	a := []Event{{Title: "Lunch", Start: start, End: end, Markers: []string{"A"}}}
	b := []Event{{Title: " lunch ", Start: start, End: end, Markers: []string{"B"}}}

	got := Merge(a, b)
	if len(got) != 1 {
		t.Fatalf("fuzzy duplicate not collapsed: %+v", got)
	}
	if !hasMarkers(got[0].Markers, "A", "B") {
		t.Errorf("markers = %v, want both A and B", got[0].Markers)
	}
}

// hasMarkers reports whether got contains exactly the wanted markers (any order).
func hasMarkers(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	set := map[string]bool{}
	for _, m := range got {
		set[m] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func TestMergeDistinctTitlesNotCollapsed(t *testing.T) {
	start := at("2026-06-10T12:00:00Z")
	a := []Event{{Title: "Lunch", Start: start, End: start.Add(time.Hour), Markers: []string{"A"}}}
	b := []Event{{Title: "Gym", Start: start, End: start.Add(time.Hour), Markers: []string{"B"}}}
	if got := Merge(a, b); len(got) != 2 {
		t.Fatalf("distinct events collapsed: %+v", got)
	}
}
