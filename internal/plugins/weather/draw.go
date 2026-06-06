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

func setFace(dc *gg.Context, size float64, bold bool) {
	if f, err := plugins.Face(size, bold); err == nil {
		dc.SetFontFace(f)
	}
}

func drawCurrent(dc *gg.Context, d Data) {
	cat, label := codeInfo(d.Code)

	setFace(dc, 34, true)
	dc.DrawStringAnchored(d.Place, 40, 60, 0, 0.5)

	drawIcon(dc, cat, 110, 210, 75)

	setFace(dc, 120, true)
	dc.DrawStringAnchored(fmt.Sprintf("%.0f°%s", d.Temp, d.TempUnit), 210, 200, 0, 0.5)

	setFace(dc, 36, false)
	dc.DrawStringAnchored(label, 40, 320, 0, 0.5)

	setFace(dc, 26, false)
	dc.DrawStringAnchored(
		fmt.Sprintf("Humidity %d%%     Wind %.0f %s", d.Humidity, d.Wind, d.WindUnit),
		40, 375, 0, 0.5)
}

func drawForecast(dc *gg.Context, d Data) {
	dc.SetLineWidth(2)
	dc.DrawLine(450, 50, 450, 430)
	dc.Stroke()

	setFace(dc, 24, true)
	dc.DrawStringAnchored("Forecast", 480, 70, 0, 0.5)

	y := 130.0
	for _, day := range d.Days {
		cat, _ := codeInfo(day.Code)
		setFace(dc, 26, true)
		dc.DrawStringAnchored(day.Name, 480, y, 0, 0.5)
		drawIcon(dc, cat, 625, y, 26)
		setFace(dc, 24, false)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f° / %.0f°", day.Hi, day.Lo), 680, y, 0, 0.5)
		y += 80
	}
}

func drawLabel(dc *gg.Context, d Data, w, h int) {
	text := d.Label
	if text == "" {
		text = "Weather"
	}
	setFace(dc, 18, false)
	dc.DrawStringAnchored(text, float64(w)-20, float64(h)-20, 1, 0.5)
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
