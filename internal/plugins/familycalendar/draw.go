package familycalendar

import (
	"fmt"
	"math"
	"strings"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/plugins/weather"
)

const (
	margin = 15
	calW   = 580 // width for the calendar column
	lineH  = 26  // event row height
	dayGap = 8   // extra space above a day header
)

func setFace(dc *gg.Context, fs *plugins.FontSet, size float64, style plugins.FontStyle) {
	if f, err := fs.Face(size, style); err == nil {
		dc.SetFontFace(f)
	}
}

func drawAgenda(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	// Draw divider
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(1.5)
	dc.DrawLine(calW, 0, calW, float64(h)-30)
	dc.Stroke()

	// 1. Calendar column
	drawCalendar(dc, fs, d, calW, h)

	// 2. Weather column
	if d.Weather != nil {
		drawWeather(dc, fs, *d.Weather, calW, 0, w-calW)
	}

	// 3. Footer
	drawFooter(dc, fs, d, w, h)
}

func drawCalendar(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	x := float64(margin)
	y := float64(margin)
	maxY := float64(h) - 40

	if len(d.Days) == 0 {
		setFace(dc, fs, 26, plugins.StyleSans)
		dc.DrawStringAnchored("No upcoming events", float64(w)/2, float64(h)/2, 0.5, 0.5)
		return
	}

	timeW := 75.0
	markerW := 35.0
	titleX := x + timeW + markerW - 10
	titleMaxW := float64(w) - margin - titleX

	for _, day := range d.Days {
		if y+lineH+20 > maxY {
			return
		}
		y += dayGap

		// Day Header: white text on black background
		setFace(dc, fs, 16, plugins.StyleTitle)
		headerText := strings.ToUpper(day.Header)
		tw, th := dc.MeasureString(headerText)
		dc.SetRGB(0, 0, 0)
		dc.DrawRectangle(x, y, tw+10, th+6)
		dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(headerText, x+5, y+th+1)
		y += th + 10

		for _, it := range day.Items {
			if y+lineH > maxY {
				return
			}
			dc.SetRGB(0, 0, 0)
			// Time
			setFace(dc, fs, 16, plugins.StyleSans)
			dc.DrawString(it.Time, x, y+16)

			// Markers (Badges)
			if it.Markers != "" {
				setFace(dc, fs, 14, plugins.StyleTitle)
				mw, mh := dc.MeasureString(it.Markers)
				dc.DrawRectangle(x+timeW, y+2, mw+6, mh+4)
				dc.Stroke()
				dc.DrawString(it.Markers, x+timeW+3, y+16)
			}

			// Title
			setFace(dc, fs, 18, plugins.StyleSans)
			dc.DrawString(truncate(dc, it.Title, titleMaxW), titleX, y+17)
			y += lineH
		}
	}
}

func drawWeather(dc *gg.Context, fs *plugins.FontSet, wd weather.Data, x, y, w int) {
	dc.SetRGB(0, 0, 0)
	cx := float64(x) + float64(w)/2
	currY := float64(y) + 30

	// Location label
	if wd.Place != "" {
		setFace(dc, fs, 12, plugins.StyleTitle)
		dc.DrawStringAnchored(strings.ToUpper(wd.Place), cx, currY, 0.5, 0.5)
	}
	currY += 15
	dc.SetLineWidth(1.5)
	dc.DrawLine(float64(x)+10, currY, float64(x+w)-10, currY)
	dc.Stroke()
	currY += 30

	// Current Temp & Icon
	cat := weatherCodeInfo(wd.Code)
	drawWeatherIcon(dc, cat, cx-35, currY, 40)
	setFace(dc, fs, 36, plugins.StyleTitle)
	dc.DrawStringAnchored(fmt.Sprintf("%.0f°", wd.Temp), cx+25, currY, 0.5, 0.5)
	currY += 35

	// Condition
	setFace(dc, fs, 14, plugins.StyleSans)
	dc.DrawStringAnchored(wd.CondLabel, cx, currY, 0.5, 0.5)
	currY += 20

	// Details: today's high/low and rain chance from the forecast.
	setFace(dc, fs, 12, plugins.StyleSans)
	if len(wd.Days) > 0 {
		today := wd.Days[0]
		dc.DrawStringAnchored(fmt.Sprintf("min %.0f° · max %.0f°", today.Lo, today.Hi), cx, currY, 0.5, 0.5)
		currY += 20
		dc.DrawStringAnchored(fmt.Sprintf("☂ %d%% rain", today.PrecipPct), cx, currY, 0.5, 0.5)
	}
	currY += 30

	// Tomorrow Forecast
	if len(wd.Days) > 1 {
		tom := wd.Days[1]
		dc.SetLineWidth(0.5)
		dc.DrawLine(float64(x)+10, currY, float64(x+w)-10, currY)
		dc.Stroke()
		currY += 15
		setFace(dc, fs, 11, plugins.StyleTitle)
		dc.DrawStringAnchored("TOMORROW", float64(x)+15, currY, 0, 0.5)
		currY += 25

		tcat := weatherCodeInfo(tom.Code)
		drawWeatherIcon(dc, tcat, float64(x)+25, currY, 24)
		setFace(dc, fs, 20, plugins.StyleTitle)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f°", tom.Hi), float64(x)+50, currY, 0, 0.5)
		currY += 18
		setFace(dc, fs, 11, plugins.StyleSans)
		dc.DrawStringAnchored(fmt.Sprintf("☂ %d%% · %s", tom.PrecipPct, tom.Heading), float64(x)+50, currY, 0, 0.5)
	}
}

func drawFooter(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(1.5)
	dc.DrawLine(0, float64(h)-30, float64(w), float64(h)-30)
	dc.Stroke()

	setFace(dc, fs, 11, plugins.StyleSans)
	dc.DrawStringAnchored("Letter in box = person · empty = all", 15, float64(h)-15, 0, 0.5)

	timeFmt := "3:04 PM"
	if d.Use24h {
		timeFmt = "15:04"
	}
	updated := "Updated: " + d.Now.Format("Jan 2, 2006") + ", " + d.Now.Format(timeFmt)
	dc.DrawStringAnchored(updated, float64(w)-15, float64(h)-15, 1, 0.5)
}

type weatherCategory int

const (
	wCatClear weatherCategory = iota
	wCatPartly
	wCatCloudy
	wCatFog
	wCatRain
	wCatSnow
	wCatStorm
)

func weatherCodeInfo(code int) weatherCategory {
	switch {
	case code == 0:
		return wCatClear
	case code == 1, code == 2:
		return wCatPartly
	case code == 3:
		return wCatCloudy
	case code == 45, code == 48:
		return wCatFog
	case code >= 51 && code <= 57:
		return wCatRain
	case code >= 61 && code <= 67:
		return wCatRain
	case code >= 71 && code <= 77:
		return wCatSnow
	case code >= 80 && code <= 82:
		return wCatRain
	case code >= 85 && code <= 86:
		return wCatSnow
	case code >= 95:
		return wCatStorm
	default:
		return wCatCloudy
	}
}

func drawWeatherIcon(dc *gg.Context, cat weatherCategory, cx, cy, s float64) {
	dc.SetRGB(0, 0, 0)
	switch cat {
	case wCatClear:
		drawSun(dc, cx, cy, s)
	case wCatPartly:
		drawSun(dc, cx-s*0.25, cy-s*0.2, s*0.6)
		drawCloud(dc, cx+s*0.1, cy+s*0.15, s)
	case wCatCloudy:
		drawCloud(dc, cx, cy, s)
	case wCatFog:
		drawCloud(dc, cx, cy-s*0.1, s*0.9)
		drawWeatherLines(dc, cx, cy+s*0.45, s, 3)
	case wCatRain:
		drawCloud(dc, cx, cy-s*0.15, s)
		drawDrops(dc, cx, cy+s*0.4, s)
	case wCatSnow:
		drawCloud(dc, cx, cy-s*0.15, s)
		drawFlakes(dc, cx, cy+s*0.4, s)
	case wCatStorm:
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

func drawWeatherLines(dc *gg.Context, cx, cy, s float64, n int) {
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

// truncate shortens s with an ellipsis so it fits within maxW pixels.
func truncate(dc *gg.Context, s string, maxW float64) string {
	if w, _ := dc.MeasureString(s); w <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		cand := string(r) + "…"
		if w, _ := dc.MeasureString(cand); w <= maxW {
			return cand
		}
	}
	return "…"
}
