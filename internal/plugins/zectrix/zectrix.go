// Package zectrix is a built-in plugin that shows current weather conditions
// and a short forecast in a layout designed for small panels such as the
// Zectrix Note v4. It reuses the weather plugin's settings and Open-Meteo data
// model and only draws differently.
package zectrix

import (
	"context"
	"fmt"
	"image"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/plugins/weather"
)

func init() {
	plugins.Register(New())
}

// Plugin draws the weather plugin's data model with its own layout. DataModel
// and DefaultRefresh come from the embedded weather plugin.
type Plugin struct {
	*weather.Plugin
}

// Data is the weather plugin's data model, which Render draws.
type Data = weather.Data

// DayForecast is one day of the weather plugin's forecast.
type DayForecast = weather.DayForecast

// New returns a zectrix plugin backed by the public Open-Meteo API.
func New() *Plugin {
	return &Plugin{Plugin: weather.New()}
}

// Type returns the registry key.
func (p *Plugin) Type() string { return "zectrix" }

// Title returns the human-friendly plugin name.
func (p *Plugin) Title() string { return "ZectrixView" }

// Render draws the weather screen to an RGBA image.
func (p *Plugin) Render(_ context.Context, in plugins.RenderInput, raw any) (*image.RGBA, error) {
	d, ok := raw.(Data)
	if !ok {
		return nil, fmt.Errorf("zectrix: invalid data model")
	}
	d.FooterReserveRight = in.FooterReserveRight
	img := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
	dc := gg.NewContextForRGBA(img)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)

	l := newLayout(in.Width, in.Height)
	drawCurrent(dc, in.Fonts, d, l)
	drawForecast(dc, in.Fonts, d, l)
	drawLabel(dc, in.Fonts, d, l)
	return img, nil
}
