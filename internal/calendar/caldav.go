package calendar

import (
	"context"
	"fmt"
	"net/http"
	"time"

	ical "github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
)

// DefaultCalDAVEndpoint is Apple iCloud's CalDAV entry point.
const DefaultCalDAVEndpoint = "https://caldav.icloud.com"

// caldavSource fetches events from a CalDAV account (Apple iCloud or a generic
// server) using HTTP basic auth (for iCloud, an app-specific password).
type caldavSource struct {
	acc Account
}

func (s *caldavSource) client() (*caldav.Client, error) {
	cfg := s.acc.CalDAV
	if cfg.Username == "" || cfg.Password == "" {
		return nil, fmt.Errorf("caldav: missing username or password")
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultCalDAVEndpoint
	}
	httpClient := webdav.HTTPClientWithBasicAuth(&http.Client{Timeout: 20 * time.Second}, cfg.Username, cfg.Password)
	return caldav.NewClient(httpClient, endpoint)
}

// discoverCalendars lists the calendars in the account's home set.
func (s *caldavSource) discoverCalendars(ctx context.Context, c *caldav.Client) ([]caldav.Calendar, error) {
	principal, err := c.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return nil, fmt.Errorf("caldav: find principal: %w", err)
	}
	homeSet, err := c.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("caldav: find calendar home set: %w", err)
	}
	cals, err := c.FindCalendars(ctx, homeSet)
	if err != nil {
		return nil, fmt.Errorf("caldav: find calendars: %w", err)
	}
	return cals, nil
}

// eventProps are the VEVENT properties we request. iCloud ignores <C:allprop>
// and <C:allcomp> (it returns an empty component), so we must list properties
// explicitly. Keep this in sync with what mapICalEvent reads.
var eventProps = []string{
	"UID", "SUMMARY", "DTSTART", "DTEND", "DURATION",
	"LOCATION", "STATUS", "RRULE", "RECURRENCE-ID",
}

// buildCalendarQuery builds a calendar-query REPORT for VEVENTs overlapping w,
// requesting the explicit property set. We do not use server-side <C:expand>:
// iCloud returns empty data for it, so recurring events come back as their
// master component (recurrence expansion is a follow-up; see ROADMAP).
func buildCalendarQuery(w Window) *caldav.CalendarQuery {
	return &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name: "VCALENDAR",
			Comps: []caldav.CalendarCompRequest{{
				Name:  "VEVENT",
				Props: eventProps,
			}},
		},
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{{
				Name:  "VEVENT",
				Start: w.From,
				End:   w.To,
			}},
		},
	}
}

func (s *caldavSource) Fetch(ctx context.Context, w Window) ([]Event, error) {
	c, err := s.client()
	if err != nil {
		return nil, err
	}

	// Resolve which calendar paths to query. With nothing selected, default to a
	// minimal set: the first discovered event calendar (CalDAV has no reliable
	// notion of a "primary" calendar via this library, so we cannot match
	// Google's primary exactly). Users pick the calendars they want on the
	// account page.
	paths := s.acc.CalDAV.CalendarPaths
	if len(paths) == 0 {
		cals, err := s.discoverCalendars(ctx, c)
		if err != nil {
			return nil, err
		}
		for _, cal := range cals {
			if supportsEvents(cal) {
				paths = append(paths, cal.Path)
				break
			}
		}
	}

	query := buildCalendarQuery(w)

	var marker []string
	if s.acc.Marker != "" {
		marker = []string{s.acc.Marker}
	}

	var out []Event
	for _, path := range paths {
		objs, err := c.QueryCalendar(ctx, path, query)
		if err != nil {
			return nil, fmt.Errorf("caldav: query %q: %w", path, err)
		}
		for _, obj := range objs {
			if obj.Data == nil {
				continue
			}
			for _, ev := range obj.Data.Events() {
				ev := ev
				out = append(out, expandEvent(&ev, w, s.acc.ID, marker)...)
			}
		}
	}
	return out, nil
}

// supportsEvents reports whether a calendar advertises VEVENT support (or makes
// no claim, in which case we include it).
func supportsEvents(cal caldav.Calendar) bool {
	if len(cal.SupportedComponentSet) == 0 {
		return true
	}
	for _, comp := range cal.SupportedComponentSet {
		if comp == "VEVENT" {
			return true
		}
	}
	return false
}

// mapICalEvent converts a parsed VEVENT into our normalized Event.
// expandEvent maps a VEVENT to one or more normalized events. A non-recurring
// event yields a single event; a recurring one (RRULE) is expanded client-side
// into its occurrences within the window, since iCloud will not expand
// server-side. Each occurrence keeps the series UID, so distinct start times
// remain distinct rows and still deduplicate across accounts.
func expandEvent(ev *ical.Event, w Window, accountID int64, marker []string) []Event {
	base, ok := mapICalEvent(ev, accountID, marker)
	if !ok {
		return nil
	}
	set, err := ev.RecurrenceSet(time.Local)
	if err != nil || set == nil {
		// Not recurring (or an unparseable rule): emit the event as-is.
		return []Event{base}
	}
	dur := base.End.Sub(base.Start)
	var out []Event
	for _, start := range set.Between(w.From, w.To, true) {
		e := base
		e.Start = start
		if dur > 0 {
			e.End = start.Add(dur)
		} else {
			e.End = start
		}
		out = append(out, e)
	}
	return out
}

func mapICalEvent(ev *ical.Event, accountID int64, marker []string) (Event, bool) {
	if status, _ := ev.Status(); status == ical.EventCancelled {
		return Event{}, false
	}
	startProp := ev.Props.Get(ical.PropDateTimeStart)
	if startProp == nil {
		return Event{}, false
	}
	start, err := ev.DateTimeStart(time.Local)
	if err != nil {
		return Event{}, false
	}
	end, _ := ev.DateTimeEnd(time.Local)

	title, _ := ev.Props.Text(ical.PropSummary)
	if title == "" {
		title = "(no title)"
	}
	uid, _ := ev.Props.Text(ical.PropUID)
	location, _ := ev.Props.Text(ical.PropLocation)
	status, _ := ev.Props.Text(ical.PropStatus)

	return Event{
		UID:       uid,
		Title:     title,
		Start:     start,
		End:       end,
		AllDay:    startProp.ValueType() == ical.ValueDate,
		Location:  location,
		Status:    status,
		AccountID: accountID,
		Markers:   marker,
	}, true
}
