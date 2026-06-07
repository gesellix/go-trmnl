package quote

import (
	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

func setFace(dc *gg.Context, size float64, bold bool) {
	if f, err := plugins.Face(size, bold); err == nil {
		dc.SetFontFace(f)
	}
}

// quoteSize picks a font size that keeps longer quotes readable on the panel.
func quoteSize(n int) float64 {
	switch {
	case n <= 60:
		return 52
	case n <= 140:
		return 42
	case n <= 260:
		return 34
	default:
		return 28
	}
}

func drawQuote(dc *gg.Context, d Data, w, h int) {
	fw, fh := float64(w), float64(h)
	cx := fw / 2

	text := d.Text
	if text != "" {
		text = "“" + text + "”" // curly double quotes
	}

	size := quoteSize(len(d.Text))
	setFace(dc, size, true)
	width := fw - 160
	lines := dc.WordWrap(text, width)

	// Measure a representative line height.
	_, lineH := dc.MeasureString("Ag")
	lineH *= 1.35
	blockH := lineH * float64(len(lines))

	authorH := 0.0
	if d.Author != "" {
		authorH = 70
	}
	// Vertically center the quote+author block, leaving room for the footer.
	top := (fh - blockH - authorH) / 2
	if top < 60 {
		top = 60
	}

	dc.SetRGB(0, 0, 0)
	y := top + lineH*0.8
	for _, ln := range lines {
		dc.DrawStringAnchored(ln, cx, y, 0.5, 0.5)
		y += lineH
	}

	if d.Author != "" {
		setFace(dc, 28, false)
		dc.DrawStringAnchored("— "+d.Author, cx, y+24, 0.5, 0.5)
	}

	// Footer: label on the left, provider attribution on the right.
	setFace(dc, 18, false)
	if d.Label != "" {
		dc.DrawStringAnchored(d.Label, 40, fh-20, 0, 0.5)
	}
	if d.Attribution != "" {
		dc.DrawStringAnchored(d.Attribution, fw-40, fh-20, 1, 0.5)
	}
}
