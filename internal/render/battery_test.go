package render_test

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/gesellix/go-trmnl/internal/render"
)

func blank() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, render.Width, render.Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

func darkPixels(img *image.RGBA, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if (cr+cg+cb)/3 < 0x7fff {
				n++
			}
		}
	}
	return n
}

// The indicator must stay inside the space plugins are asked to keep free,
// otherwise it covers their footer text.
func TestBatteryIndicatorStaysInReservedArea(t *testing.T) {
	img := blank()
	render.DrawBatteryIndicator(img, render.BatteryFull, false)

	reserved := image.Rect(render.Width-render.BatteryFooterWidth, render.Height-40, render.Width, render.Height)
	if got := darkPixels(img, reserved); got == 0 {
		t.Fatal("nothing drawn in the reserved area")
	}
	outside := darkPixels(img, img.Bounds()) - darkPixels(img, reserved)
	if outside != 0 {
		t.Errorf("%d dark pixels outside the reserved area", outside)
	}
}

// More charge must mean more ink: the bars are the whole point.
func TestBatteryLevelsDifferVisibly(t *testing.T) {
	prev := -1
	for _, level := range []render.BatteryLevel{
		render.BatteryUnknown, render.BatteryEmpty, render.BatteryLow,
		render.BatteryMedium, render.BatteryFull,
	} {
		img := blank()
		render.DrawBatteryIndicator(img, level, false)
		n := darkPixels(img, img.Bounds())
		if n <= prev {
			t.Errorf("level %d drew %d dark pixels, not more than the previous %d", level, n, prev)
		}
		prev = n
	}
}

func TestBatteryChargingAddsBolt(t *testing.T) {
	plain, charging := blank(), blank()
	render.DrawBatteryIndicator(plain, render.BatteryLow, false)
	render.DrawBatteryIndicator(charging, render.BatteryLow, true)
	if darkPixels(plain, plain.Bounds()) == darkPixels(charging, charging.Bounds()) {
		t.Error("charging state did not change the drawing")
	}
}

func TestBatteryLevelFor(t *testing.T) {
	for _, tc := range []struct {
		volts float64
		want  render.BatteryLevel
	}{
		{0, render.BatteryUnknown},
		{4.15, render.BatteryFull},
		{3.9, render.BatteryFull},
		{3.75, render.BatteryMedium},
		{3.55, render.BatteryLow},
		{3.42, render.BatteryEmpty},
		{3.1, render.BatteryEmpty},
	} {
		if got := render.BatteryLevelFor(tc.volts); got != tc.want {
			t.Errorf("BatteryLevelFor(%.2f) = %d, want %d", tc.volts, got, tc.want)
		}
	}
}

// The image is cached by content hash, so small voltage moves within a level
// must not produce a new image.
func TestBatteryDrawingIsStableWithinALevel(t *testing.T) {
	a, b := blank(), blank()
	render.DrawBatteryIndicator(a, render.BatteryLevelFor(3.71), false)
	render.DrawBatteryIndicator(b, render.BatteryLevelFor(3.88), false)
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatal("3.71 V and 3.88 V drew different images although both are 'medium'")
		}
	}
}
