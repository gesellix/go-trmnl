// Package familycalendar renders an agenda of upcoming events merged from the
// family's calendar accounts (configured under /admin/calendar).
package familycalendar

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/calendar"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

// Plugin renders the merged family agenda. It reads events from the shared
// calendar cache via the injected service; svc may be nil (renders empty).
type Plugin struct {
	svc *calendar.Service
}

// New constructs the plugin. Pass nil to render an empty agenda (e.g. the CLI
// or render golden tests without a database).
func New(svc *calendar.Service) *Plugin { return &Plugin{svc: svc} }

// Type returns the registry key.
func (p *Plugin) Type() string { return "familycalendar" }

// Title returns the human-friendly plugin name.
func (p *Plugin) Title() string { return "Family Calendar" }

// DefaultRefresh returns the cache TTL hint for rendered agenda screens.
func (p *Plugin) DefaultRefresh() time.Duration { return 15 * time.Minute }

type settings struct {
	Accounts  []int64 `json:"accounts"`   // account IDs to include; empty = all
	Days      int     `json:"days"`       // window length in days (default 14)
	MaxEvents int     `json:"max_events"` // cap on events shown (default 30)
	Label     string  `json:"label"`      // optional title
	Use24h    bool    `json:"use_24h"`
}

// Item is one rendered agenda entry.
type Item struct {
	Time    string
	Title   string
	Markers string
}

// Day groups items under a date header.
type Day struct {
	Header string
	Items  []Item
}

// Data is the render model.
type Data struct {
	Label string
	Days  []Day
}

// DataModel fetches the merged agenda for the selected accounts and window.
func (p *Plugin) DataModel(ctx context.Context, in plugins.RenderInput) (any, error) {
	var s settings
	if len(in.Settings) > 0 {
		if err := json.Unmarshal(in.Settings, &s); err != nil {
			return nil, fmt.Errorf("familycalendar: bad settings: %w", err)
		}
	}
	days := s.Days
	if days <= 0 {
		days = 14
	}
	maxEvents := s.MaxEvents
	if maxEvents <= 0 {
		maxEvents = 30
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}

	var events []calendar.Event
	if p.svc != nil {
		start := startOfDay(now)
		w := calendar.Window{From: start, To: start.AddDate(0, 0, days)}
		evs, err := p.svc.Agenda(s.Accounts, w, maxEvents)
		if err != nil {
			return nil, fmt.Errorf("familycalendar: agenda: %w", err)
		}
		events = evs
	}

	return Data{Label: s.Label, Days: groupByDay(events, s.Use24h)}, nil
}

// Render draws the day-grouped agenda to an RGBA image.
func (p *Plugin) Render(_ context.Context, in plugins.RenderInput, raw any) (*image.RGBA, error) {
	d, ok := raw.(Data)
	if !ok {
		return nil, fmt.Errorf("familycalendar: unexpected data type %T", raw)
	}
	img := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
	dc := gg.NewContextForRGBA(img)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	drawAgenda(dc, d, in.Width, in.Height)
	return img, nil
}

// groupByDay turns sorted events into day-grouped render items.
func groupByDay(events []calendar.Event, use24h bool) []Day {
	var days []Day
	var cur *Day
	var curKey string
	for _, e := range events {
		local := e.Start.Local()
		key := local.Format("2006-01-02")
		if cur == nil || key != curKey {
			days = append(days, Day{Header: local.Format("Mon, Jan 2")})
			cur = &days[len(days)-1]
			curKey = key
		}
		cur.Items = append(cur.Items, Item{
			Time:    eventTime(e, use24h),
			Title:   e.Title,
			Markers: strings.Join(e.Markers, " "),
		})
	}
	return days
}

func eventTime(e calendar.Event, use24h bool) string {
	if e.AllDay {
		return "all day"
	}
	layout := "3:04 PM"
	if use24h {
		layout = "15:04"
	}
	return e.Start.Local().Format(layout)
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
