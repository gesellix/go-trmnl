package calendar

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/store"
)

type fakeSource struct {
	acc Account
	evs []Event
}

func (f *fakeSource) Fetch(_ context.Context, _ Window) ([]Event, error) {
	out := make([]Event, 0, len(f.evs))
	for _, e := range f.evs {
		e.AccountID = f.acc.ID // markers are re-applied from the account in Agenda
		out = append(out, e)
	}
	return out, nil
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestServiceSyncAndAgenda(t *testing.T) {
	st := openStore(t)
	a, _ := st.CreateCalendarAccount(string(ProviderGoogle), "Mom", "A", `{"refresh_token":"r"}`, 0)
	b, _ := st.CreateCalendarAccount(string(ProviderGoogle), "Dad", "B", `{"refresh_token":"r"}`, 0)

	shared := Event{UID: "shared", Title: "Dinner", Start: at("2026-06-10T18:00:00Z"), End: at("2026-06-10T20:00:00Z")}
	svc := NewService(st, NewGoogleOAuth("", "", ""))
	svc.newSource = func(acc Account, _ sourceDeps) (Source, error) {
		switch acc.Marker {
		case "A":
			return &fakeSource{acc: acc, evs: []Event{
				shared,
				{UID: "solo", Title: "Gym", Start: at("2026-06-10T07:00:00Z"), End: at("2026-06-10T08:00:00Z")},
			}}, nil
		default:
			return &fakeSource{acc: acc, evs: []Event{shared}}, nil
		}
	}

	now := at("2026-06-09T00:00:00Z")
	if err := svc.SyncDue(context.Background(), now); err != nil {
		t.Fatalf("SyncDue: %v", err)
	}

	// Both accounts recorded a sync time.
	for _, id := range []int64{a.ID, b.ID} {
		got, _ := st.GetCalendarAccount(id)
		if !got.LastSync.Valid {
			t.Errorf("account %d not marked synced", id)
		}
	}

	// Agenda over a wide window: shared event deduped (markers A+B) + Mom's solo.
	w := Window{From: at("2026-06-01T00:00:00Z"), To: at("2026-07-01T00:00:00Z")}
	evs, err := svc.Agenda(nil, w, 0)
	if err != nil {
		t.Fatalf("Agenda: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("agenda len = %d, want 2: %+v", len(evs), evs)
	}
	// Sorted by start: Gym (07:00) before Dinner (18:00).
	if evs[0].Title != "Gym" || evs[1].Title != "Dinner" {
		t.Fatalf("agenda order wrong: %+v", evs)
	}
	if !hasMarkers(evs[1].Markers, "A", "B") {
		t.Errorf("shared event markers = %v, want A and B", evs[1].Markers)
	}

	// Not due yet: a second SyncDue right after must skip (last_sync unchanged).
	before, _ := st.GetCalendarAccount(a.ID)
	if err := svc.SyncDue(context.Background(), now.Add(time.Minute)); err != nil {
		t.Fatalf("SyncDue 2: %v", err)
	}
	after, _ := st.GetCalendarAccount(a.ID)
	if before.LastSync.Int64 != after.LastSync.Int64 {
		t.Errorf("account re-synced before its interval elapsed")
	}
}

func TestNextWakeClamps(t *testing.T) {
	st := openStore(t)
	svc := NewService(st, NewGoogleOAuth("", "", ""))

	// No accounts -> default 15m.
	if got := svc.NextWake(); got != 15*time.Minute {
		t.Errorf("NextWake (no accounts) = %v, want 15m", got)
	}
	// 12h interval clamps to the 1h ceiling.
	_, _ = st.CreateCalendarAccount(string(ProviderGoogle), "x", "X", `{}`, 12*60*60)
	if got := svc.NextWake(); got != time.Hour {
		t.Errorf("NextWake = %v, want 1h ceiling", got)
	}
	// A 30s interval clamps to the 1m floor.
	_, _ = st.CreateCalendarAccount(string(ProviderGoogle), "y", "Y", `{}`, 30)
	if got := svc.NextWake(); got != time.Minute {
		t.Errorf("NextWake = %v, want 1m floor", got)
	}
}
