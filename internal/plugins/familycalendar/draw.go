package familycalendar

import (
	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

const (
	margin     = 28
	timeColW   = 95 // width reserved for the time column
	markerColW = 70 // width reserved for marker badges
	lineH      = 30 // event row height
	dayGap     = 14 // extra space above a day header
	headerH    = 32 // day header row height
)

func setFace(dc *gg.Context, size float64, bold bool) {
	if f, err := plugins.Face(size, bold); err == nil {
		dc.SetFontFace(f)
	}
}

func drawAgenda(dc *gg.Context, d Data, w, h int) {
	x := float64(margin)
	y := float64(margin)
	maxY := float64(h) - margin

	// Title.
	if d.Label != "" {
		setFace(dc, 30, true)
		dc.DrawString(d.Label, x, y+24)
		y += 44
	}

	if len(d.Days) == 0 {
		setFace(dc, 26, false)
		dc.DrawStringAnchored("No upcoming events", float64(w)/2, float64(h)/2, 0.5, 0.5)
		return
	}

	titleX := x + timeColW + markerColW
	titleMaxW := float64(w) - margin - titleX

	for _, day := range d.Days {
		// Stop if there's no room for a header plus one row.
		if y+headerH+lineH > maxY {
			drawMore(dc, x, y, maxY)
			return
		}
		y += dayGap
		setFace(dc, 22, true)
		dc.DrawString(day.Header, x, y+18)
		y += headerH
		// Separator line under the header.
		dc.SetLineWidth(1)
		dc.DrawLine(x, y-6, float64(w)-margin, y-6)
		dc.Stroke()

		for _, it := range day.Items {
			if y+lineH > maxY {
				drawMore(dc, x, y, maxY)
				return
			}
			setFace(dc, 20, false)
			dc.DrawString(it.Time, x, y+20)
			if it.Markers != "" {
				setFace(dc, 18, true)
				dc.DrawString(it.Markers, x+timeColW, y+20)
			}
			setFace(dc, 20, false)
			dc.DrawString(truncate(dc, it.Title, titleMaxW), titleX, y+20)
			y += lineH
		}
	}
}

// drawMore prints a subtle overflow hint at the bottom.
func drawMore(dc *gg.Context, x, y, maxY float64) {
	if y+18 > maxY {
		y = maxY - 4
	}
	setFace(dc, 18, false)
	dc.DrawString("…", x, y+14)
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
