package familycalendar

import (
	"context"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/calendar"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

func ev(start, title string, allDay bool, markers ...string) calendar.Event {
	t, err := time.Parse(time.RFC3339, start)
	if err != nil {
		panic(err)
	}
	return calendar.Event{Title: title, Start: t, End: t.Add(time.Hour), AllDay: allDay, Markers: markers}
}

func TestDataModelNilServiceIsEmpty(t *testing.T) {
	p := New(nil)
	raw, err := p.DataModel(context.Background(), plugins.RenderInput{
		Now:      time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC),
		Settings: []byte(`{"label":"Family","days":7}`),
	})
	if err != nil {
		t.Fatalf("DataModel: %v", err)
	}
	d := raw.(Data)
	if d.Label != "Family" {
		t.Errorf("label = %q, want Family", d.Label)
	}
	if len(d.Days) != 0 {
		t.Errorf("expected no days with nil service, got %d", len(d.Days))
	}
}

func TestDataModelBadSettings(t *testing.T) {
	if _, err := New(nil).DataModel(context.Background(), plugins.RenderInput{Settings: []byte(`{bad`)}); err == nil {
		t.Error("expected error for invalid settings JSON")
	}
}

func TestGroupByDay(t *testing.T) {
	// Use UTC for deterministic day grouping regardless of the test host's zone.
	time.Local = time.UTC
	// Two events on day 10, one on day 11 -> two day groups.
	events := []calendar.Event{
		ev("2026-06-10T09:00:00Z", "Standup", false, "A"),
		ev("2026-06-10T18:00:00Z", "Dinner", false, "A", "B"),
		ev("2026-06-11T00:00:00Z", "Holiday", true, "B"),
	}
	days := groupByDay(events, true)
	if len(days) != 2 {
		t.Fatalf("got %d day groups, want 2: %+v", len(days), days)
	}
	if len(days[0].Items) != 2 || len(days[1].Items) != 1 {
		t.Fatalf("items per day wrong: %+v", days)
	}
	if days[0].Items[0].Time != "09:00" {
		t.Errorf("24h time = %q, want 09:00", days[0].Items[0].Time)
	}
	if days[0].Items[1].Markers != "A B" {
		t.Errorf("markers = %q, want 'A B'", days[0].Items[1].Markers)
	}
	if days[1].Items[0].Time != "all day" {
		t.Errorf("all-day time = %q", days[1].Items[0].Time)
	}
}

func TestRenderProducesPanel(t *testing.T) {
	p := New(nil)
	d := Data{
		Label: "Family",
		Days: []Day{
			{Header: "Tue, Jun 9", Items: []Item{
				{Time: "09:00", Title: "Standup", Markers: "A"},
				{Time: "all day", Title: "Conference with a very long title that should be truncated to fit", Markers: "A B"},
			}},
		},
	}
	img, err := p.Render(context.Background(), plugins.RenderInput{Width: 800, Height: 480}, d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("bounds = %v, want 800x480", b)
	}
}

func TestRenderEmptyState(t *testing.T) {
	img, err := New(nil).Render(context.Background(), plugins.RenderInput{Width: 800, Height: 480}, Data{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if img == nil {
		t.Fatal("nil image")
	}
}
