// Package render converts rendered images into the 1-bit formats the TRMNL
// firmware decodes (BMP3 and PNG), and provides simple image helpers.
//
// The standard library and golang.org/x/image/bmp cannot emit 1-bpp BMPs, so
// this package contains a small custom 1-bit BMP3 encoder.
package render

import (
	"encoding/binary"
	"image"
	"image/color"
	"io"
)

const (
	fileHeaderSize = 14
	infoHeaderSize = 40
	paletteSize    = 8 // two BGRA entries
	pixelOffset    = fileHeaderSize + infoHeaderSize + paletteSize
)

// rowStride returns the 4-byte-aligned byte length of one 1-bpp scanline.
func rowStride(width int) int { return ((width + 31) / 32) * 4 }

// isWhite reports whether a pixel should map to the white palette entry (bit 1)
// using a luminance threshold. Callers that have already reduced the image to
// black/white (e.g. after dithering) get a stable result.
func isWhite(c color.Color) bool {
	r, g, b, _ := c.RGBA() // 16-bit per channel
	// Rec. 601 luma, compared at the 8-bit midpoint (128 << 8).
	luma := (299*r + 587*g + 114*b) / 1000
	return luma >= 128<<8
}

// EncodeBMP1 writes img as a 1-bit (monochrome) BMP3. Pixels are thresholded to
// black/white via luminance. Rows are stored bottom-up with MSB-first packing,
// matching the BMP format the firmware expects.
func EncodeBMP1(w io.Writer, img image.Image) error {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	stride := rowStride(width)
	pixelDataSize := stride * height
	fileSize := pixelOffset + pixelDataSize

	header := make([]byte, pixelOffset)
	// BITMAPFILEHEADER
	header[0], header[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(header[2:], uint32(fileSize))
	binary.LittleEndian.PutUint32(header[10:], pixelOffset)
	// BITMAPINFOHEADER
	binary.LittleEndian.PutUint32(header[14:], infoHeaderSize)
	binary.LittleEndian.PutUint32(header[18:], uint32(width))
	binary.LittleEndian.PutUint32(header[22:], uint32(height)) // positive => bottom-up
	binary.LittleEndian.PutUint16(header[26:], 1)              // planes
	binary.LittleEndian.PutUint16(header[28:], 1)              // bits per pixel
	binary.LittleEndian.PutUint32(header[30:], 0)              // BI_RGB
	binary.LittleEndian.PutUint32(header[34:], uint32(pixelDataSize))
	binary.LittleEndian.PutUint32(header[38:], 2835) // ~72 DPI X
	binary.LittleEndian.PutUint32(header[42:], 2835) // ~72 DPI Y
	binary.LittleEndian.PutUint32(header[46:], 2)    // colors used
	binary.LittleEndian.PutUint32(header[50:], 2)    // important colors
	// Color table (BGRA): index 0 = black, index 1 = white.
	// black already zero at header[54:58]
	header[58], header[59], header[60], header[61] = 0xFF, 0xFF, 0xFF, 0x00
	if _, err := w.Write(header); err != nil {
		return err
	}

	row := make([]byte, stride)
	for y := height - 1; y >= 0; y-- { // bottom-up
		for i := range row {
			row[i] = 0
		}
		for x := 0; x < width; x++ {
			if isWhite(img.At(b.Min.X+x, b.Min.Y+y)) {
				row[x/8] |= 1 << (7 - uint(x%8)) // MSB-first
			}
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
