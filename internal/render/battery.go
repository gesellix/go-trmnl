package render

import (
	"image"
	"image/draw"
)

// Battery indicator geometry, in pixels. BatteryFooterWidth is what a plugin
// should keep free at the bottom right when the indicator is drawn.
const (
	batteryWidth  = 34
	batteryHeight = 16
	batteryMargin = 15 // distance from the panel edges, matching the plugin footers

	// BatteryFooterWidth is the horizontal space the indicator occupies,
	// measured from the right edge of the panel: the icon with its right-hand
	// nub and the white plate around it, plus a gap to whatever the plugin
	// draws next to it.
	BatteryFooterWidth = batteryMargin + batteryWidth + 3 + 12
)

// BatteryLevel is a coarse charge level. The indicator deliberately has only a
// few steps: the rendered image is cached by content hash, so a value that
// changes on every poll would force a download and a full e-ink refresh each
// time, costing more battery than the indicator is worth.
type BatteryLevel int

// Charge levels, from "no telemetry yet" to a full battery.
const (
	BatteryUnknown BatteryLevel = iota // no bars
	BatteryEmpty                       // one bar
	BatteryLow                         // two bars
	BatteryMedium                      // three bars
	BatteryFull                        // four bars
)

// BatteryLevelFor maps a single-cell LiPo voltage to a coarse level. The
// thresholds follow the discharge curve's flat middle: most of the useful life
// sits between 3.9 V and 3.6 V, and below 3.4 V the drop is steep.
func BatteryLevelFor(volts float64) BatteryLevel {
	switch {
	case volts <= 0:
		return BatteryUnknown
	case volts >= 3.9:
		return BatteryFull
	case volts >= 3.7:
		return BatteryMedium
	case volts >= 3.5:
		return BatteryLow
	default:
		return BatteryEmpty
	}
}

// BatteryState folds everything the indicator draws into one comparable
// number, so a cached render can be invalidated exactly when the picture would
// change and not on every voltage wobble.
func BatteryState(level BatteryLevel, charging bool) int {
	n := int(level) * 2
	if charging {
		n++
	}
	return n
}

// DrawBatteryIndicator draws a battery outline filled according to level in the
// bottom-right corner of img, with a bolt across it while charging. An unknown
// level draws the empty outline, so the footer's layout does not jump around
// once the first telemetry arrives. It is a no-op for a nil image.
func DrawBatteryIndicator(img draw.Image, level BatteryLevel, charging bool) {
	if img == nil {
		return
	}
	b := img.Bounds()
	x1 := b.Max.X - batteryMargin
	y1 := b.Max.Y - batteryMargin + batteryHeight/2
	x0 := x1 - batteryWidth
	y0 := y1 - batteryHeight

	ink := image.NewUniform(black)
	paper := image.NewUniform(white)

	// Clear the area first: the indicator sits on top of the plugin's output,
	// and 1-bit dithering turns any overlap into noise.
	draw.Draw(img, image.Rect(x0-3, y0-3, x1+6, y1+3), paper, image.Point{}, draw.Src)

	// Outline, plus the nub on the right.
	rect(img, ink, x0, y0, x1, y0+2)
	rect(img, ink, x0, y1-2, x1, y1)
	rect(img, ink, x0, y0, x0+2, y1)
	rect(img, ink, x1-2, y0, x1, y1)
	rect(img, ink, x1, y0+4, x1+3, y1-4)

	// While charging, a bolt replaces the bars: at this size the two together
	// are unreadable, and the level is climbing anyway.
	if charging {
		drawBolt(img, ink, (x0+x1)/2, (y0+y1)/2)
		return
	}

	// Bars: four cells inside the outline, filled from the left.
	bars := int(level)
	if level == BatteryUnknown {
		bars = 0
	}
	const gap = 2
	cell := (batteryWidth - 4 - gap*5) / 4
	for i := 0; i < bars; i++ {
		bx := x0 + 2 + gap + i*(cell+gap)
		rect(img, ink, bx, y0+4, bx+cell, y1-4)
	}
}

// rect fills the half-open rectangle [x0,x1) x [y0,y1).
func rect(img draw.Image, src image.Image, x0, y0, x1, y1 int) {
	draw.Draw(img, image.Rect(x0, y0, x1, y1), src, image.Point{}, draw.Src)
}

// drawBolt draws a lightning bolt centred on (cx, cy): an upper wedge leaning
// right, a wide waist, and a lower wedge leaning left. Hand-plotted row by row,
// because at ten pixels wide anything generated looks like a smudge.
func drawBolt(img draw.Image, ink image.Image, cx, cy int) {
	// Thin strokes: a wedge leaning down-left, a jog, then the same again.
	// Anything fatter turns into a blob inside a 34x16 outline.
	rows := []struct{ dy, x0, x1 int }{
		{-5, 2, 5}, {-4, 1, 4}, {-3, 0, 3}, {-2, -1, 2},
		{-1, -2, 4}, {0, -4, 2},
		{1, -2, 1}, {2, -3, 0}, {3, -4, -1}, {4, -5, -2},
	}
	for _, r := range rows {
		rect(img, ink, cx+r.x0, cy+r.dy, cx+r.x1, cy+r.dy+1)
	}
}
