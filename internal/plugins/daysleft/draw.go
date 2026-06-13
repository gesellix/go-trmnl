package daysleft

import (
	"fmt"
	"math"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

const margin = 28

func setFace(dc *gg.Context, fs *plugins.FontSet, size float64, style plugins.FontStyle) {
	if f, err := fs.Face(size, style); err == nil {
		dc.SetFontFace(f)
	}
}

func draw(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	fw, fh := float64(w), float64(h)

	// --- Top: two big numbers with a dotted separator bar each ---
	colTop, colBot := 48.0, 150.0
	drawDottedBar(dc, margin+6, colTop, colBot)
	drawDottedBar(dc, fw/2+margin, colTop, colBot)

	dc.SetRGB(0, 0, 0)
	setFace(dc, fs, 110, plugins.StyleTitle)
	dc.DrawStringAnchored(fmt.Sprintf("%d", d.Passed), margin+28, 110, 0, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("%d", d.Left), fw/2+margin+22, 110, 0, 0.5)

	setFace(dc, fs, 30, plugins.StyleSans)
	dc.DrawStringAnchored("Days Passed", margin+30, 178, 0, 0.5)
	dc.DrawStringAnchored("Days Left", fw/2+margin+24, 178, 0, 0.5)

	if d.Label != "" {
		setFace(dc, fs, 24, plugins.StyleSans)
		dc.DrawStringAnchored(d.Label, fw-margin, 60, 1, 0.5)
	}

	// --- Year dot grid ---
	gridTop := 220.0
	gridBot := fh - 56
	drawGrid(dc, d, margin, gridTop, fw-margin, gridBot)

	// --- Footer ---
	footerY := fh - 30
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(1)
	dc.DrawLine(margin, footerY-20, fw-margin, footerY-20)
	dc.Stroke()
	drawCalendarGlyph(dc, margin+10, footerY-2, 18)
	setFace(dc, fs, 20, plugins.StyleTitle)
	dc.DrawStringAnchored("Days Left This Year", margin+34, footerY, 0, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("%d", d.Year), fw-margin, footerY, 1, 0.5)
}

// drawDottedBar draws a vertical column of small dots between y0 and y1.
func drawDottedBar(dc *gg.Context, x, y0, y1 float64) {
	dc.SetRGB(0.35, 0.35, 0.35)
	step := 7.0
	for y := y0; y <= y1; y += step {
		dc.DrawCircle(x, y, 1.6)
		dc.Fill()
	}
}

// drawGrid lays out one dot per day in a balanced grid within the given box.
// Past days are drawn darker, future days lighter, and today solid black.
func drawGrid(dc *gg.Context, d Data, x0, y0, x1, y1 float64) {
	usableW := x1 - x0
	usableH := y1 - y0

	// Pick the row count whose cells are closest to square.
	cols, rows := gridShape(d.Total, usableW, usableH)
	cellW := usableW / float64(cols)
	cellH := usableH / float64(rows)
	r := math.Min(cellW, cellH) * 0.30

	for i := 0; i < d.Total; i++ {
		row := i / cols
		col := i % cols
		cx := x0 + (float64(col)+0.5)*cellW
		cy := y0 + (float64(row)+0.5)*cellH

		switch {
		case i == d.TodayIndex:
			dc.SetRGB(0, 0, 0)
			dc.DrawCircle(cx, cy, r*1.35)
		case i < d.TodayIndex:
			dc.SetRGB(0.4, 0.4, 0.4) // past
			dc.DrawCircle(cx, cy, r)
		default:
			dc.SetRGB(0.72, 0.72, 0.72) // future
			dc.DrawCircle(cx, cy, r)
		}
		dc.Fill()
	}
}

// gridShape chooses (cols, rows) for n cells in a w x h box, preferring cells
// that are close to square.
func gridShape(n int, w, h float64) (cols, rows int) {
	best := math.MaxFloat64
	cols, rows = n, 1
	for tryRows := 4; tryRows <= 14; tryRows++ {
		tryCols := (n + tryRows - 1) / tryRows
		cw := w / float64(tryCols)
		ch := h / float64(tryRows)
		diff := math.Abs(cw - ch)
		if diff < best {
			best = diff
			cols, rows = tryCols, tryRows
		}
	}
	return cols, rows
}

// drawCalendarGlyph draws a tiny calendar icon centered vertically near (x, y).
func drawCalendarGlyph(dc *gg.Context, x, y, s float64) {
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(1.5)
	top := y - s
	dc.DrawRectangle(x, top, s, s)
	dc.Stroke()
	dc.DrawLine(x, top+s*0.32, x+s, top+s*0.32) // header bar
	dc.Stroke()
	dc.DrawLine(x+s*0.3, top-s*0.12, x+s*0.3, top+s*0.12) // rings
	dc.DrawLine(x+s*0.7, top-s*0.12, x+s*0.7, top+s*0.12)
	dc.Stroke()
}
