package store_test

import (
	"errors"
	"testing"

	"github.com/gesellix/go-trmnl/internal/store"
)

func TestCalendarAccountCRUD(t *testing.T) {
	st := openTest(t)

	// Defaults: empty config -> "{}", non-positive interval -> 12h.
	a, err := st.CreateCalendarAccount("google", "Mom", "M", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if a.ConfigJSON != "{}" {
		t.Errorf("default config = %q, want {}", a.ConfigJSON)
	}
	if a.RefreshInterval != 12*60*60 {
		t.Errorf("default refresh interval = %d, want 43200", a.RefreshInterval)
	}

	got, err := st.GetCalendarAccount(a.ID)
	if err != nil || got.Name != "Mom" || got.Provider != "google" || got.Marker != "M" {
		t.Fatalf("GetCalendarAccount: %+v err=%v", got, err)
	}

	// Update editable fields + config + interval.
	if err := st.UpdateCalendarAccount(a.ID, "Mum", "Mu", `{"email":"mom@x"}`, 3600); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetCalendarAccount(a.ID)
	if got.Name != "Mum" || got.Marker != "Mu" || got.ConfigJSON != `{"email":"mom@x"}` || got.RefreshInterval != 3600 {
		t.Fatalf("update not applied: %+v", got)
	}

	// Config-only persist (token refresh path).
	if err := st.SetCalendarAccountConfig(a.ID, `{"token":"new"}`); err != nil {
		t.Fatal(err)
	}
	if got, _ = st.GetCalendarAccount(a.ID); got.ConfigJSON != `{"token":"new"}` {
		t.Errorf("SetCalendarAccountConfig: %q", got.ConfigJSON)
	}

	// Sync bookkeeping: error then success clears it.
	_ = st.SetCalendarAccountSync(a.ID, 100, "boom")
	if got, _ = st.GetCalendarAccount(a.ID); got.LastSync.Int64 != 100 || got.LastError.String != "boom" {
		t.Errorf("sync error not recorded: %+v", got)
	}
	_ = st.SetCalendarAccountSync(a.ID, 200, "")
	if got, _ = st.GetCalendarAccount(a.ID); got.LastSync.Int64 != 200 || got.LastError.Valid {
		t.Errorf("sync success did not clear error: %+v", got)
	}

	if accs, _ := st.ListCalendarAccounts(); len(accs) != 1 {
		t.Errorf("ListCalendarAccounts = %d, want 1", len(accs))
	}

	if err := st.DeleteCalendarAccount(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetCalendarAccount(a.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("account still present after delete: %v", err)
	}
}

func TestCalendarEventsReplaceAndList(t *testing.T) {
	st := openTest(t)
	a, _ := st.CreateCalendarAccount("google", "Dad", "D", "", 0)
	b, _ := st.CreateCalendarAccount("google", "Kid", "K", "", 0)

	evsA := []store.CalendarEvent{
		{UID: "u1", Title: "Standup", StartAt: 1000, EndAt: 2000},
		{UID: "u2", Title: "Lunch", StartAt: 5000, EndAt: 6000, AllDay: true, Location: "Cafe"},
		// Duplicate (account_id, uid, start_at) is ignored by INSERT OR IGNORE.
		{UID: "u1", Title: "Standup dup", StartAt: 1000, EndAt: 2000},
	}
	if err := st.ReplaceCalendarEvents(a.ID, evsA, 9999); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceCalendarEvents(b.ID, []store.CalendarEvent{{UID: "u3", Title: "Soccer", StartAt: 1500, EndAt: 1600}}, 9999); err != nil {
		t.Fatal(err)
	}

	// Window [0,3000) over all accounts: u1 (a) and u3 (b), ordered by start.
	all, err := st.ListCalendarEvents(nil, 0, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].UID != "u1" || all[1].UID != "u3" {
		t.Fatalf("window/order wrong: %+v", all)
	}

	// Filter by account A only.
	onlyA, _ := st.ListCalendarEvents([]int64{a.ID}, 0, 10000)
	if len(onlyA) != 2 {
		t.Fatalf("account filter = %d events, want 2 (dup ignored)", len(onlyA))
	}
	// All-day + location round-trip.
	var lunch *store.CalendarEvent
	for i := range onlyA {
		if onlyA[i].UID == "u2" {
			lunch = &onlyA[i]
		}
	}
	if lunch == nil || !lunch.AllDay || lunch.Location != "Cafe" {
		t.Errorf("all-day/location not preserved: %+v", lunch)
	}

	// Replace replaces (not appends): account A now has a single event.
	if err := st.ReplaceCalendarEvents(a.ID, []store.CalendarEvent{{UID: "uX", Title: "New", StartAt: 1, EndAt: 2}}, 10000); err != nil {
		t.Fatal(err)
	}
	if onlyA, _ = st.ListCalendarEvents([]int64{a.ID}, 0, 10000); len(onlyA) != 1 || onlyA[0].UID != "uX" {
		t.Fatalf("replace did not replace: %+v", onlyA)
	}

	// Deleting an account cascades its cached events.
	if err := st.DeleteCalendarAccount(b.ID); err != nil {
		t.Fatal(err)
	}
	if evs, _ := st.ListCalendarEvents([]int64{b.ID}, 0, 10000); len(evs) != 0 {
		t.Errorf("events not cascaded on account delete: %d", len(evs))
	}
}
