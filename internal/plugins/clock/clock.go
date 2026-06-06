// Package clock is a built-in plugin that renders the current time and date.
// It performs no IO, which makes it a good end-to-end smoke screen.
package clock

import (
	"context"
	"encoding/json"
	"image"
	"time"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

func init() { plugins.Register(&Plugin{}) }

// Plugin renders a clock face.
type Plugin struct{}

// Type returns the registry key.
func (p *Plugin) Type() string { return "clock" }

// Title returns the human-friendly plugin name.
func (p *Plugin) Title() string { return "Clock" }

// DefaultRefresh returns the cache TTL hint for rendered clock screens.
func (p *Plugin) DefaultRefresh() time.Duration { return time.Minute }

// settings configures the clock screen.
type settings struct {
	Timezone string `json:"timezone"` // IANA name; empty == server local
	Use24h   bool   `json:"use_24h"`
	Label    string `json:"label"`
}

type data struct {
	Time  string
	Date  string
	Label string
}

// DataModel computes the time/date strings to display.
func (p *Plugin) DataModel(_ context.Context, in plugins.RenderInput) (any, error) {
	var s settings
	if len(in.Settings) > 0 {
		_ = json.Unmarshal(in.Settings, &s)
	}
	now := in.Now
	if s.Timezone != "" {
		if loc, err := time.LoadLocation(s.Timezone); err == nil {
			now = now.In(loc)
		}
	}
	timeFmt := "3:04 PM"
	if s.Use24h {
		timeFmt = "15:04"
	}
	return data{
		Time:  now.Format(timeFmt),
		Date:  now.Format("Monday, January 2"),
		Label: s.Label,
	}, nil
}

// Render draws the clock face to an RGBA image.
func (p *Plugin) Render(_ context.Context, in plugins.RenderInput, raw any) (*image.RGBA, error) {
	d, _ := raw.(data)
	img := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
	dc := gg.NewContextForRGBA(img)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)

	cx, cy := float64(in.Width)/2, float64(in.Height)/2

	if d.Label != "" {
		if f, err := plugins.Face(36, false); err == nil {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(d.Label, cx, cy-150, 0.5, 0.5)
		}
	}
	if f, err := plugins.Face(140, true); err == nil {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(d.Time, cx, cy, 0.5, 0.5)
	}
	if f, err := plugins.Face(44, false); err == nil {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(d.Date, cx, cy+120, 0.5, 0.5)
	}
	return img, nil
}
