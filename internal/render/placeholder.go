package render

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Standard TRMNL panel dimensions.
const (
	Width  = 800
	Height = 480
)

var (
	black = color.RGBA{0, 0, 0, 255}
	white = color.RGBA{255, 255, 255, 255}
)

// Placeholder renders a simple white screen with a black border and the given
// centered text lines. It is used before a device has any screens assigned.
func Placeholder(lines []string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(white), image.Point{}, draw.Src)

	// Border, 4px thick.
	border := image.NewUniform(black)
	for t := 0; t < 4; t++ {
		draw.Draw(img, image.Rect(t, t, Width-t, t+1), border, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(t, Height-t-1, Width-t, Height-t), border, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(t, t, t+1, Height-t), border, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(Width-t-1, t, Width-t, Height-t), border, image.Point{}, draw.Src)
	}

	face := basicfont.Face7x13
	lineHeight := face.Metrics().Height.Ceil() + 6
	totalHeight := lineHeight * len(lines)
	startY := (Height-totalHeight)/2 + face.Metrics().Ascent.Ceil()

	for i, line := range lines {
		w := font.MeasureString(face, line).Ceil()
		d := &font.Drawer{
			Dst:  img,
			Src:  border,
			Face: face,
			Dot:  fixed.P((Width-w)/2, startY+i*lineHeight),
		}
		d.DrawString(line)
	}
	return img
}
