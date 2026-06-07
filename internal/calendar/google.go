package calendar

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// googleSource fetches events from one Google account via the Calendar API.
type googleSource struct {
	acc  Account
	deps sourceDeps

	// svcFn overrides service construction in tests. When nil, the service is
	// built from the account's OAuth token (and the returned TokenSource is used
	// to persist refreshed tokens). When set in tests, it returns a nil
	// TokenSource so token persistence is skipped.
	svcFn func(ctx context.Context) (*gcal.Service, oauth2.TokenSource, error)
}

func (s *googleSource) service(ctx context.Context) (*gcal.Service, oauth2.TokenSource, error) {
	if s.svcFn != nil {
		return s.svcFn(ctx)
	}
	oauth, err := s.deps.googleOAuthFor(s.acc.Config.OAuthClientID)
	if err != nil {
		return nil, nil, err
	}
	ts := oauth.cfg.TokenSource(ctx, configToToken(s.acc.Config))
	svc, err := gcal.NewService(ctx, option.WithTokenSource(ts))
	return svc, ts, err
}

func (s *googleSource) Fetch(ctx context.Context, w Window) ([]Event, error) {
	svc, ts, err := s.service(ctx)
	if err != nil {
		return nil, fmt.Errorf("google service: %w", err)
	}

	ids := s.acc.Config.CalendarIDs
	if len(ids) == 0 {
		ids = []string{"primary"}
	}

	var out []Event
	for _, id := range ids {
		call := svc.Events.List(id).
			SingleEvents(true). // expand recurring events server-side
			OrderBy("startTime").
			ShowDeleted(false).
			TimeMin(w.From.Format(time.RFC3339)).
			TimeMax(w.To.Format(time.RFC3339)).
			MaxResults(2500)
		err := call.Pages(ctx, func(page *gcal.Events) error {
			for _, it := range page.Items {
				if e, ok := mapGoogleEvent(it, s.acc); ok {
					out = append(out, e)
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("events.list %q: %w", id, err)
		}
	}

	s.persistToken(ts)
	return out, nil
}

// persistToken writes back a refreshed token so the next sync starts fresh.
func (s *googleSource) persistToken(ts oauth2.TokenSource) {
	if ts == nil || s.deps.onGoogleToken == nil {
		return
	}
	tok, err := ts.Token()
	if err != nil {
		return
	}
	cfg := s.acc.Config
	if tok.AccessToken == cfg.AccessToken && tok.Expiry.Equal(cfg.Expiry) {
		return // unchanged
	}
	cfg.AccessToken = tok.AccessToken
	cfg.TokenType = tok.TokenType
	cfg.Expiry = tok.Expiry
	if tok.RefreshToken != "" {
		cfg.RefreshToken = tok.RefreshToken
	}
	_ = s.deps.onGoogleToken(s.acc.ID, cfg)
}

func mapGoogleEvent(it *gcal.Event, acc Account) (Event, bool) {
	if it.Status == "cancelled" {
		return Event{}, false
	}
	start, allDay, ok := parseGoogleTime(it.Start)
	if !ok {
		return Event{}, false
	}
	end, _, _ := parseGoogleTime(it.End)
	title := it.Summary
	if title == "" {
		title = "(no title)"
	}
	var markers []string
	if acc.Marker != "" {
		markers = []string{acc.Marker}
	}
	return Event{
		UID:       it.ICalUID,
		Title:     title,
		Start:     start,
		End:       end,
		AllDay:    allDay,
		Location:  it.Location,
		Status:    it.Status,
		AccountID: acc.ID,
		Markers:   markers,
	}, true
}

// parseGoogleTime parses a Calendar API EventDateTime, distinguishing timed
// events (DateTime, RFC3339) from all-day events (Date, yyyy-mm-dd).
func parseGoogleTime(t *gcal.EventDateTime) (when time.Time, allDay, ok bool) {
	if t == nil {
		return time.Time{}, false, false
	}
	if t.DateTime != "" {
		tt, err := time.Parse(time.RFC3339, t.DateTime)
		if err != nil {
			return time.Time{}, false, false
		}
		return tt, false, true
	}
	if t.Date != "" {
		// All-day: anchor at local midnight so day grouping is correct for the
		// server's timezone.
		tt, err := time.ParseInLocation("2006-01-02", t.Date, time.Local)
		if err != nil {
			return time.Time{}, false, false
		}
		return tt, true, true
	}
	return time.Time{}, false, false
}
