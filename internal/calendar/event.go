// Package calendar provides a provider-agnostic layer over family calendars:
// it fetches events from multiple accounts (Google now, CalDAV next), merges and
// deduplicates them, and caches them for the familycalendar screen plugin.
package calendar

import (
	"sort"
	"strings"
	"time"
)

// Event is a normalized calendar event, independent of its source provider.
type Event struct {
	UID      string
	Title    string
	Start    time.Time
	End      time.Time
	AllDay   bool
	Location string
	Status   string

	// AccountID is the account this event was first seen on. Markers holds the
	// badge of every account the event appears on (more than one after a shared
	// event is deduplicated across family members).
	AccountID int64
	Markers   []string
}

// Window is an inclusive-start, exclusive-end time range to fetch.
type Window struct {
	From, To time.Time
}

// dedupKey identifies the same event across accounts. Recurring instances keep
// distinct keys because their start times differ. Events with a shared iCalUID
// (e.g. an invite both parents accepted) collapse; otherwise we fall back to a
// normalized title plus start/end.
func dedupKey(e Event) string {
	if uid := strings.TrimSpace(e.UID); uid != "" {
		return "uid:" + uid + "|" + e.Start.UTC().Format(time.RFC3339)
	}
	return "t:" + strings.ToLower(strings.TrimSpace(e.Title)) +
		"|" + e.Start.UTC().Format(time.RFC3339) +
		"|" + e.End.UTC().Format(time.RFC3339)
}

// Merge concatenates event sets, sorts them by start time, and deduplicates
// across accounts. When duplicates collapse, the surviving event accumulates
// every source's marker (deduplicated, order-preserving).
func Merge(sets ...[]Event) []Event {
	var all []Event
	for _, s := range sets {
		all = append(all, s...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].Start.Equal(all[j].Start) {
			return all[i].Start.Before(all[j].Start)
		}
		return all[i].Title < all[j].Title
	})

	out := make([]Event, 0, len(all))
	byKey := make(map[string]int, len(all)) // key -> index in out
	for _, e := range all {
		k := dedupKey(e)
		if idx, ok := byKey[k]; ok {
			out[idx].Markers = addMarkers(out[idx].Markers, e.Markers)
			continue
		}
		byKey[k] = len(out)
		out = append(out, e)
	}
	return out
}

// addMarkers appends markers not already present, preserving order.
func addMarkers(dst, extra []string) []string {
	for _, m := range extra {
		if m == "" {
			continue
		}
		seen := false
		for _, d := range dst {
			if d == m {
				seen = true
				break
			}
		}
		if !seen {
			dst = append(dst, m)
		}
	}
	return dst
}
