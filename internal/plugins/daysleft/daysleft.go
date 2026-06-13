// Package daysleft is a built-in plugin that shows how much of the current year
// has passed and how much remains, with a dot grid of the whole year. Modeled on
// the TRMNL "Days Left This Year" screen. It performs no IO.
package daysleft

import (
	"context"
	"encoding/json"
	"image"
	"time"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

func init() { plugins.Register(&Plugin{}) }

// Plugin renders the year-progress screen.
type Plugin struct{}

// Type returns the registry key.
func (p *Plugin) Type() string { return "days_left_year" }

// Title returns the human-friendly plugin name.
func (p *Plugin) Title() string { return "Days Left This Year" }

// DefaultRefresh returns the cache TTL hint. The content only changes once a
// day; the content-hash cache dedupes identical re-renders within the day.
func (p *Plugin) DefaultRefresh() time.Duration { return time.Hour }

// settings configures the screen.
type settings struct {
	Timezone string `json:"timezone"` // IANA name; empty == server local
	Label    string `json:"label"`
}

// Data is the render model. Exported so Render can be golden-tested with a
// hand-built value.
type Data struct {
	Year       int
	Passed     int // days elapsed including today
	Left       int // days remaining (Total - Passed)
	Total      int // days in the year (365 or 366)
	TodayIndex int // zero-based index of today within the year
	Label      string
}

// DataModel computes the year-progress numbers for "today" in the configured
// timezone.
func (p *Plugin) DataModel(_ context.Context, in plugins.RenderInput) (any, error) {
	var s settings
	if len(in.Settings) > 0 {
		_ = json.Unmarshal(in.Settings, &s)
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	if s.Timezone != "" {
		if loc, err := time.LoadLocation(s.Timezone); err == nil {
			now = now.In(loc)
		}
	}

	year := now.Year()
	total := daysInYear(year)
	doy := now.YearDay() // 1..total

	return Data{
		Year:       year,
		Passed:     doy,
		Left:       total - doy,
		Total:      total,
		TodayIndex: doy - 1,
		Label:      s.Label,
	}, nil
}

// Render draws the screen to an RGBA image.
func (p *Plugin) Render(_ context.Context, in plugins.RenderInput, raw any) (*image.RGBA, error) {
	d, _ := raw.(Data)
	img := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
	dc := gg.NewContextForRGBA(img)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	draw(dc, in.Fonts, d, in.Width, in.Height)
	return img, nil
}

// daysInYear returns 366 for leap years, else 365.
func daysInYear(year int) int {
	if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
		return 366
	}
	return 365
}
