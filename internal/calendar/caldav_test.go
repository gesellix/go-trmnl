package calendar

import (
	"strings"
	"testing"
	"time"

	ical "github.com/emersion/go-ical"
)

const sampleICS = "BEGIN:VCALENDAR\r\n" +
	"VERSION:2.0\r\n" +
	"PRODID:-//test//EN\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:timed-1\r\n" +
	"DTSTAMP:20260601T000000Z\r\n" +
	"SUMMARY:Standup\r\n" +
	"DTSTART:20260610T090000Z\r\n" +
	"DTEND:20260610T091500Z\r\n" +
	"END:VEVENT\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:allday-1\r\n" +
	"DTSTAMP:20260601T000000Z\r\n" +
	"SUMMARY:Holiday\r\n" +
	"LOCATION:Home\r\n" +
	"DTSTART;VALUE=DATE:20260611\r\n" +
	"DTEND;VALUE=DATE:20260612\r\n" +
	"END:VEVENT\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:cancelled-1\r\n" +
	"DTSTAMP:20260601T000000Z\r\n" +
	"SUMMARY:Dropped\r\n" +
	"STATUS:CANCELLED\r\n" +
	"DTSTART:20260610T100000Z\r\n" +
	"DTEND:20260610T110000Z\r\n" +
	"END:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestBuildCalendarQueryUsesExplicitProps(t *testing.T) {
	// iCloud ignores allprop/allcomp and returns empty data for server-side
	// expand, so the query must list properties explicitly and not use those.
	q := buildCalendarQuery(Window{From: at("2026-06-01T00:00:00Z"), To: at("2026-07-01T00:00:00Z")})

	if len(q.CompRequest.Comps) != 1 || q.CompRequest.Comps[0].Name != "VEVENT" {
		t.Fatalf("expected a single VEVENT comp request, got %+v", q.CompRequest.Comps)
	}
	ve := q.CompRequest.Comps[0]
	if ve.AllProps || ve.AllComps || ve.Expand != nil || q.CompRequest.AllProps || q.CompRequest.AllComps || q.CompRequest.Expand != nil {
		t.Errorf("query must not use allprop/allcomp/expand: %+v", q.CompRequest)
	}
	want := map[string]bool{"SUMMARY": false, "DTSTART": false, "UID": false}
	for _, p := range ve.Props {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for p, seen := range want {
		if !seen {
			t.Errorf("query missing required prop %q", p)
		}
	}
	if !q.CompFilter.Comps[0].Start.Equal(at("2026-06-01T00:00:00Z")) {
		t.Errorf("time-range start not set: %+v", q.CompFilter)
	}
}

const recurringICS = "BEGIN:VCALENDAR\r\n" +
	"VERSION:2.0\r\n" +
	"PRODID:-//test//EN\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:weekly-1\r\n" +
	"DTSTAMP:20260501T000000Z\r\n" +
	"SUMMARY:Standup\r\n" +
	"DTSTART:20260504T090000Z\r\n" + // Monday 2026-05-04, in the past relative to the window
	"DTEND:20260504T091500Z\r\n" +
	"RRULE:FREQ=WEEKLY;BYDAY=MO\r\n" +
	"END:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestExpandEventRecurring(t *testing.T) {
	time.Local = time.UTC

	cal, err := ical.NewDecoder(strings.NewReader(recurringICS)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	events := cal.Events()
	if len(events) != 1 {
		t.Fatalf("want 1 master event, got %d", len(events))
	}

	// Window: June 2026. The weekly series started in May, so without expansion
	// the master (May 4) would be outside the window and nothing would show.
	w := Window{From: at("2026-06-01T00:00:00Z"), To: at("2026-07-01T00:00:00Z")}
	got := expandEvent(&events[0], w, 9, []string{"X"})

	// Mondays in June 2026: 1, 8, 15, 22, 29 -> five occurrences.
	if len(got) != 5 {
		t.Fatalf("expanded to %d occurrences, want 5: %+v", len(got), got)
	}
	for _, e := range got {
		if e.Title != "Standup" || e.UID != "weekly-1" || e.AccountID != 9 {
			t.Errorf("instance fields wrong: %+v", e)
		}
		if e.Start.Before(w.From) || !e.Start.Before(w.To) {
			t.Errorf("instance %v outside window", e.Start)
		}
		if e.End.Sub(e.Start) != 15*time.Minute {
			t.Errorf("instance duration = %v, want 15m", e.End.Sub(e.Start))
		}
	}
	if !got[0].Start.Equal(at("2026-06-01T09:00:00Z")) {
		t.Errorf("first occurrence = %v, want 2026-06-01T09:00Z", got[0].Start)
	}
}

func TestExpandEventNonRecurring(t *testing.T) {
	cal, _ := ical.NewDecoder(strings.NewReader(sampleICS)).Decode()
	for _, ev := range cal.Events() {
		ev := ev
		got := expandEvent(&ev, Window{From: at("2026-06-01T00:00:00Z"), To: at("2026-07-01T00:00:00Z")}, 1, nil)
		// Non-recurring events map to exactly one event (cancelled -> zero).
		if status, _ := ev.Status(); status == ical.EventCancelled {
			if len(got) != 0 {
				t.Errorf("cancelled event should expand to 0, got %d", len(got))
			}
		} else if len(got) != 1 {
			t.Errorf("non-recurring event expanded to %d, want 1", len(got))
		}
	}
}

func TestMapICalEvent(t *testing.T) {
	time.Local = time.UTC // deterministic all-day grouping

	cal, err := ical.NewDecoder(strings.NewReader(sampleICS)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	byUID := map[string]Event{}
	for _, ev := range cal.Events() {
		ev := ev
		e, ok := mapICalEvent(&ev, 5, []string{"A"})
		if !ok {
			continue
		}
		byUID[e.UID] = e
	}

	// Cancelled event filtered out.
	if _, ok := byUID["cancelled-1"]; ok {
		t.Error("cancelled event should be dropped")
	}
	if len(byUID) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(byUID), byUID)
	}

	timed := byUID["timed-1"]
	if timed.Title != "Standup" || timed.AllDay {
		t.Errorf("timed event wrong: %+v", timed)
	}
	if !timed.Start.Equal(at("2026-06-10T09:00:00Z")) {
		t.Errorf("timed start = %v", timed.Start)
	}
	if timed.AccountID != 5 || len(timed.Markers) != 1 || timed.Markers[0] != "A" {
		t.Errorf("account/markers not stamped: %+v", timed)
	}

	allday := byUID["allday-1"]
	if !allday.AllDay {
		t.Errorf("all-day not detected: %+v", allday)
	}
	if allday.Location != "Home" {
		t.Errorf("location = %q", allday.Location)
	}
}
