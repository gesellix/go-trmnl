package render

import "image"

// Mode selects how a source image is reduced to pure black/white.
type Mode int

const (
	// Threshold maps each pixel to black or white at the 50% luminance point.
	Threshold Mode = iota
	// FloydSteinberg applies Floyd-Steinberg error diffusion, which looks much
	// better for photographic or gradient content on a 1-bit panel.
	FloydSteinberg
)

// ParseMode resolves a mode name; unknown values fall back to FloydSteinberg.
func ParseMode(s string) Mode {
	if s == "threshold" {
		return Threshold
	}
	return FloydSteinberg
}

// luma8 returns the Rec. 601 luminance of a pixel as an 8-bit value (0..255).
func luma8(img image.Image, x, y int) float64 {
	r, g, b, _ := img.At(x, y).RGBA() // 16-bit channels
	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256.0
}

// Monochrome reduces src to a *image.Gray whose pixels are exactly 0 (black) or
// 255 (white), using the requested mode. The result is deterministic for a
// given src and mode, which makes it suitable for content hashing.
func Monochrome(src image.Image, mode Mode) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewGray(image.Rect(0, 0, w, h))

	// Grayscale working buffer in 0..255.
	buf := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			buf[y*w+x] = luma8(src, b.Min.X+x, b.Min.Y+y)
		}
	}

	if mode == Threshold {
		for i, v := range buf {
			if v >= 128 {
				out.Pix[i] = 255
			}
		}
		return out
	}

	// Floyd-Steinberg error diffusion.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			old := buf[i]
			var newv float64
			if old >= 128 {
				newv = 255
			}
			out.Pix[i] = uint8(newv)
			err := old - newv
			diffuse(buf, w, h, x+1, y, err*7.0/16.0)
			diffuse(buf, w, h, x-1, y+1, err*3.0/16.0)
			diffuse(buf, w, h, x, y+1, err*5.0/16.0)
			diffuse(buf, w, h, x+1, y+1, err*1.0/16.0)
		}
	}
	return out
}

func diffuse(buf []float64, w, h, x, y int, delta float64) {
	if x < 0 || x >= w || y < 0 || y >= h {
		return
	}
	buf[y*w+x] += delta
}
