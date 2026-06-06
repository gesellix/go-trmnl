package render

import (
	"image"
	"image/color"
	"image/png"
	"io"
)

// bwPalette is the 2-color palette used for 1-bit PNG output.
var bwPalette = color.Palette{color.Black, color.White}

// EncodePNG1 writes img as a small paletted (2-color) PNG. Pixels are
// thresholded to black/white via luminance, matching EncodeBMP1.
func EncodePNG1(w io.Writer, img image.Image) error {
	b := img.Bounds()
	pal := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), bwPalette)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			idx := uint8(0) // black
			if isWhite(img.At(b.Min.X+x, b.Min.Y+y)) {
				idx = 1 // white
			}
			pal.SetColorIndex(x, y, idx)
		}
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	return enc.Encode(w, pal)
}
