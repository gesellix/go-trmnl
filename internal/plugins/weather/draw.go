package weather

import (
	"fmt"
	"math"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

// category buckets a WMO weather code into a drawable icon class.
type category int

const (
	catClear category = iota
	catPartly
	catCloudy
	catFog
	catRain
	catSnow
	catStorm
)

// codeInfo maps a WMO weather code to a category and a human label.
func codeInfo(code int) (category, string) {
	switch {
	case code == 0:
		return catClear, "Clear"
	case code == 1, code == 2:
		return catPartly, "Partly cloudy"
	case code == 3:
		return catCloudy, "Overcast"
	case code == 45, code == 48:
		return catFog, "Fog"
	case code >= 51 && code <= 57:
		return catRain, "Drizzle"
	case code >= 61 && code <= 67:
		return catRain, "Rain"
	case code >= 71 && code <= 77:
		return catSnow, "Snow"
	case code >= 80 && code <= 82:
		return catRain, "Showers"
	case code >= 85 && code <= 86:
		return catSnow, "Snow showers"
	case code >= 95:
		return catStorm, "Thunderstorm"
	default:
		return catCloudy, "Cloudy"
	}
}

func setFace(dc *gg.Context, fs *plugins.FontSet, size float64, style plugins.FontStyle) {
	if f, err := fs.Face(size, style); err == nil {
		dc.SetFontFace(f)
	}
}

// drawCurrent draws the top section: big icon + temperature, condition, and a
// right-hand info column (feels-like, humidity, wind, sun times).
func drawCurrent(dc *gg.Context, fs *plugins.FontSet, d Data, w int) {
	cat, _ := codeInfo(d.Code)
	fw := float64(w)

	drawIcon(dc, cat, 120, 150, 80)

	dc.SetRGB(0, 0, 0)
	setFace(dc, fs, 120, plugins.StyleTitle)
	dc.DrawStringAnchored(fmt.Sprintf("%.0f°", d.Temp), 215, 140, 0, 0.5)

	setFace(dc, fs, 34, plugins.StyleSans)
	dc.DrawStringAnchored(d.CondLabel, 40, 235, 0, 0.5)

	// Right-hand info column, right-aligned.
	rx := fw - 40
	setFace(dc, fs, 26, plugins.StyleSans)
	dc.DrawStringAnchored(fmt.Sprintf("Feels like %.0f°%s", d.FeelsLike, d.TempUnit), rx, 60, 1, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Humidity %d%%", d.Humidity), rx, 105, 1, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Wind %.0f %s", d.Wind, d.WindUnit), rx, 150, 1, 0.5)
	if d.Sunrise != "" || d.Sunset != "" {
		dc.DrawStringAnchored(fmt.Sprintf("Sun %s - %s", d.Sunrise, d.Sunset), rx, 195, 1, 0.5)
	}
}

// drawForecast draws the Today/Tomorrow rows beneath a divider.
func drawForecast(dc *gg.Context, fs *plugins.FontSet, d Data, w int) {
	fw := float64(w)
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(2)
	dc.DrawLine(40, 290, fw-40, 290)
	dc.Stroke()

	y := 350.0
	for _, day := range d.Days {
		cat, cond := codeInfo(day.Code)
		drawIcon(dc, cat, 70, y-6, 30)

		setFace(dc, fs, 28, plugins.StyleTitle)
		dc.DrawStringAnchored(day.Heading, 115, y-14, 0, 0.5)
		setFace(dc, fs, 22, plugins.StyleSans)
		dc.DrawStringAnchored(cond, 115, y+16, 0, 0.5)

		setFace(dc, fs, 24, plugins.StyleSans)
		dc.DrawStringAnchored(fmt.Sprintf("UV %d", day.UV), 380, y, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%d%% precip", day.PrecipPct), 480, y, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("L %.0f°  H %.0f°", day.Lo, day.Hi), 640, y, 0, 0.5)

		y += 70
	}
}

// drawLabel draws the footer: a left label and the place (with no time, which
// the device does not provide here) on the right.
func drawLabel(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	text := d.Label
	if text == "" {
		text = "Weather"
	}
	dc.SetRGB(0, 0, 0)
	setFace(dc, fs, 18, plugins.StyleSans)
	dc.DrawStringAnchored(text, 40, float64(h)-20, 0, 0.5)
	if d.Place != "" {
		dc.DrawStringAnchored(d.Place, float64(w)-40, float64(h)-20, 1, 0.5)
	}
}

// drawIcon draws a filled black weather glyph centered at (cx, cy) sized by s.
func drawIcon(dc *gg.Context, cat category, cx, cy, s float64) {
	dc.SetRGB(0, 0, 0)
	switch cat {
	case catClear:
		drawSun(dc, cx, cy, s)
	case catPartly:
		drawSun(dc, cx-s*0.25, cy-s*0.2, s*0.6)
		drawCloud(dc, cx+s*0.1, cy+s*0.15, s)
	case catCloudy:
		drawCloud(dc, cx, cy, s)
	case catFog:
		drawCloud(dc, cx, cy-s*0.1, s*0.9)
		drawLines(dc, cx, cy+s*0.45, s, 3)
	case catRain:
		drawCloud(dc, cx, cy-s*0.15, s)
		drawDrops(dc, cx, cy+s*0.4, s)
	case catSnow:
		drawCloud(dc, cx, cy-s*0.15, s)
		drawFlakes(dc, cx, cy+s*0.4, s)
	case catStorm:
		drawCloud(dc, cx, cy-s*0.15, s)
		drawBolt(dc, cx, cy+s*0.35, s)
	}
}

func drawSun(dc *gg.Context, cx, cy, s float64) {
	r := s * 0.32
	dc.DrawCircle(cx, cy, r)
	dc.Fill()
	dc.SetLineWidth(s * 0.06)
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		x1, y1 := cx+math.Cos(a)*r*1.4, cy+math.Sin(a)*r*1.4
		x2, y2 := cx+math.Cos(a)*r*1.9, cy+math.Sin(a)*r*1.9
		dc.DrawLine(x1, y1, x2, y2)
	}
	dc.Stroke()
}

func drawCloud(dc *gg.Context, cx, cy, s float64) {
	r := s * 0.28
	dc.DrawCircle(cx-r*1.1, cy, r)
	dc.DrawCircle(cx+r*1.1, cy, r)
	dc.DrawCircle(cx, cy-r*0.7, r*1.2)
	dc.DrawRectangle(cx-r*1.5, cy, r*3, r*1.1)
	dc.Fill()
}

func drawDrops(dc *gg.Context, cx, cy, s float64) {
	dc.SetLineWidth(s * 0.05)
	for i := -1; i <= 1; i++ {
		x := cx + float64(i)*s*0.25
		dc.DrawLine(x, cy, x-s*0.08, cy+s*0.2)
	}
	dc.Stroke()
}

func drawFlakes(dc *gg.Context, cx, cy, s float64) {
	for i := -1; i <= 1; i++ {
		dc.DrawCircle(cx+float64(i)*s*0.25, cy+s*0.05, s*0.05)
	}
	dc.Fill()
}

func drawLines(dc *gg.Context, cx, cy, s float64, n int) {
	dc.SetLineWidth(s * 0.06)
	for i := 0; i < n; i++ {
		y := cy + float64(i)*s*0.18
		dc.DrawLine(cx-s*0.5, y, cx+s*0.5, y)
	}
	dc.Stroke()
}

func drawBolt(dc *gg.Context, cx, cy, s float64) {
	dc.MoveTo(cx+s*0.1, cy-s*0.15)
	dc.LineTo(cx-s*0.15, cy+s*0.1)
	dc.LineTo(cx, cy+s*0.1)
	dc.LineTo(cx-s*0.1, cy+s*0.35)
	dc.LineTo(cx+s*0.2, cy-s*0.02)
	dc.LineTo(cx+s*0.03, cy-s*0.02)
	dc.ClosePath()
	dc.Fill()
}
