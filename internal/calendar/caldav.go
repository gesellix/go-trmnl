package calendar

import (
	"context"
	"fmt"
)

// caldavSource is the phase-2 provider for Apple iCloud and generic CalDAV
// servers. The interface is in place so the factory and service handle the
// provider; the implementation lands with the CalDAV dependency
// (github.com/emersion/go-webdav/caldav + go-ical).
type caldavSource struct {
	acc Account
}

func (s *caldavSource) Fetch(ctx context.Context, w Window) ([]Event, error) {
	// TODO(phase 2): discover principal + calendar home set, QueryCalendar with a
	// time-range CompFilter and Expand, parse VEVENTs via go-ical, map to Event.
	return nil, fmt.Errorf("caldav provider not implemented yet")
}
