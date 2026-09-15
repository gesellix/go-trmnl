package calendar

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// mockCalendar serves a fixed Events payload for the events.list endpoint.
func mockCalendar(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/calendars/primary/events", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"items": [
				{"iCalUID":"u-timed","summary":"Standup","status":"confirmed",
				 "start":{"dateTime":"2026-06-10T09:00:00Z"},"end":{"dateTime":"2026-06-10T09:15:00Z"}},
				{"iCalUID":"u-allday","summary":"Holiday","status":"confirmed","location":"Home",
				 "start":{"date":"2026-06-11"},"end":{"date":"2026-06-12"}},
				{"iCalUID":"u-cancelled","summary":"Dropped","status":"cancelled",
				 "start":{"dateTime":"2026-06-10T10:00:00Z"},"end":{"dateTime":"2026-06-10T11:00:00Z"}},
				{"iCalUID":"u-notitle","status":"confirmed",
				 "start":{"dateTime":"2026-06-10T12:00:00Z"},"end":{"dateTime":"2026-06-10T13:00:00Z"}}
			]
		}`))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestGoogleSourceFetchMapsEvents(t *testing.T) {
	ts := mockCalendar(t)
	acc := Account{ID: 7, Provider: ProviderGoogle, Marker: "D"}
	src := &googleSource{
		acc: acc,
		svcFn: func(ctx context.Context) (*gcal.Service, oauth2.TokenSource, error) {
			svc, err := gcal.NewService(ctx, option.WithEndpoint(ts.URL), option.WithHTTPClient(http.DefaultClient))
			return svc, nil, err
		},
	}

	evs, err := src.Fetch(context.Background(), Window{From: at("2026-06-01T00:00:00Z"), To: at("2026-07-01T00:00:00Z")})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Cancelled event is dropped; three remain.
	if len(evs) != 3 {
		t.Fatalf("got %d events, want 3: %+v", len(evs), evs)
	}

	byUID := map[string]Event{}
	for _, e := range evs {
		byUID[e.UID] = e
		if e.AccountID != 7 {
			t.Errorf("AccountID = %d, want 7", e.AccountID)
		}
		if len(e.Markers) != 1 || e.Markers[0] != "D" {
			t.Errorf("markers = %v, want [D]", e.Markers)
		}
	}

	timed := byUID["u-timed"]
	if timed.Title != "Standup" || timed.AllDay {
		t.Errorf("timed event wrong: %+v", timed)
	}
	if !timed.Start.Equal(at("2026-06-10T09:00:00Z")) {
		t.Errorf("timed start = %v", timed.Start)
	}

	allday := byUID["u-allday"]
	if !allday.AllDay || allday.Location != "Home" {
		t.Errorf("all-day event wrong: %+v", allday)
	}

	if byUID["u-notitle"].Title != "(no title)" {
		t.Errorf("missing-title fallback = %q", byUID["u-notitle"].Title)
	}
}

func TestIsReauthRequired(t *testing.T) {
	wrapped := fmt.Errorf("events.list: %w", &oauth2.RetrieveError{ErrorCode: "invalid_grant"})
	if !IsReauthRequired(wrapped) {
		t.Error("wrapped invalid_grant RetrieveError should require reauth")
	}
	if IsReauthRequired(&oauth2.RetrieveError{ErrorCode: "invalid_client"}) {
		t.Error("invalid_client should not require reauth")
	}
	if IsReauthRequired(errors.New("connection refused")) || IsReauthRequired(nil) {
		t.Error("unrelated errors should not require reauth")
	}
}

// TestGoogleSourceFetchInvalidGrant drives the real token refresh against a
// token endpoint that rejects the refresh token, as Google does for expired or
// revoked grants, and checks the error survives the API client's wrapping.
func TestGoogleSourceFetchInvalidGrant(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Bad Request"}`))
	}))
	t.Cleanup(tokenSrv.Close)

	oauth := &GoogleOAuth{cfg: &oauth2.Config{
		ClientID: "id", ClientSecret: "secret",
		Endpoint: oauth2.Endpoint{TokenURL: tokenSrv.URL},
	}}
	src := &googleSource{
		acc: Account{ID: 1, Provider: ProviderGoogle, Config: GoogleConfig{
			RefreshToken: "stale", AccessToken: "old", Expiry: time.Now().Add(-time.Hour),
		}},
		deps: sourceDeps{googleOAuthFor: func(int64) (*GoogleOAuth, error) { return oauth, nil }},
	}

	_, err := src.Fetch(context.Background(), Window{From: time.Now(), To: time.Now().Add(time.Hour)})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !IsReauthRequired(err) {
		t.Errorf("IsReauthRequired(%v) = false, want true", err)
	}
}
