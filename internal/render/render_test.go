package render

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate golden test files")

// gradient builds a deterministic test image: a horizontal grayscale ramp with
// a black border, useful for exercising both threshold and dithering.
func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(x * 255 / (w - 1))
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func TestEncodeBMP1Header(t *testing.T) {
	img := gradient(800, 480)
	var buf bytes.Buffer
	if err := EncodeBMP1(&buf, Monochrome(img, Threshold)); err != nil {
		t.Fatalf("encode: %v", err)
	}
	b := buf.Bytes()

	if b[0] != 'B' || b[1] != 'M' {
		t.Fatalf("missing BM magic")
	}
	if got := binary.LittleEndian.Uint32(b[10:]); got != pixelOffset {
		t.Errorf("pixel offset = %d, want %d", got, pixelOffset)
	}
	if got := binary.LittleEndian.Uint16(b[28:]); got != 1 {
		t.Errorf("biBitCount = %d, want 1", got)
	}
	if got := int32(binary.LittleEndian.Uint32(b[22:])); got != 480 {
		t.Errorf("biHeight = %d, want 480 (positive => bottom-up)", got)
	}
	if got := binary.LittleEndian.Uint32(b[46:]); got != 2 {
		t.Errorf("biClrUsed = %d, want 2", got)
	}
	// 800px at 1bpp = 100-byte rows, already 4-aligned.
	wantSize := pixelOffset + 100*480
	if len(b) != wantSize {
		t.Errorf("file size = %d, want %d", len(b), wantSize)
	}
}

func TestRowStride(t *testing.T) {
	cases := map[int]int{1: 4, 8: 4, 32: 4, 33: 8, 800: 100}
	for w, want := range cases {
		if got := rowStride(w); got != want {
			t.Errorf("rowStride(%d) = %d, want %d", w, got, want)
		}
	}
}

func TestMonochromeDeterministic(t *testing.T) {
	img := gradient(64, 16)
	a := Monochrome(img, FloydSteinberg)
	b := Monochrome(img, FloydSteinberg)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("Floyd-Steinberg output is not deterministic")
	}
	// Every pixel must be pure black or white.
	for i, v := range a.Pix {
		if v != 0 && v != 255 {
			t.Fatalf("pixel %d = %d, want 0 or 255", i, v)
		}
	}
}

func TestProcessCachesByContentHash(t *testing.T) {
	dir := t.TempDir()
	r := NewRenderer(dir)
	img := gradient(800, 480)

	res, err := r.Process(img, FloydSteinberg)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	for _, name := range []string{res.BMPName, res.PNGName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected cached file %s: %v", name, err)
		}
	}

	// Same content -> same hash, no duplicate files.
	res2, err := r.Process(img, FloydSteinberg)
	if err != nil {
		t.Fatalf("process 2: %v", err)
	}
	if res2.Hash != res.Hash {
		t.Errorf("hash changed for identical content: %s != %s", res2.Hash, res.Hash)
	}

	// Different mode -> different reduced pixels -> different hash.
	res3, _ := r.Process(img, Threshold)
	if res3.Hash == res.Hash {
		t.Errorf("threshold and dithered gradient produced the same hash")
	}
}

func TestParseMode(t *testing.T) {
	if ParseMode("threshold") != Threshold {
		t.Error(`ParseMode("threshold") should be Threshold`)
	}
	for _, s := range []string{"floyd_steinberg", "", "anything"} {
		if ParseMode(s) != FloydSteinberg {
			t.Errorf("ParseMode(%q) should default to FloydSteinberg", s)
		}
	}
}

func TestEncodePNG1IsTwoColorPaletted(t *testing.T) {
	img := gradient(64, 32)
	var buf bytes.Buffer
	if err := EncodePNG1(&buf, Monochrome(img, Threshold)); err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := png.Decode(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	pal, ok := decoded.(*image.Paletted)
	if !ok {
		t.Fatalf("decoded PNG is %T, want *image.Paletted", decoded)
	}
	if len(pal.Palette) != 2 {
		t.Errorf("palette has %d colors, want 2", len(pal.Palette))
	}
	if b := decoded.Bounds(); b.Dx() != 64 || b.Dy() != 32 {
		t.Errorf("bounds = %v", b)
	}
}

func TestGoldenBMP(t *testing.T) {
	img := gradient(128, 64)
	var buf bytes.Buffer
	if err := EncodeBMP1(&buf, Monochrome(img, FloydSteinberg)); err != nil {
		t.Fatalf("encode: %v", err)
	}
	goldenPath := filepath.Join("testdata", "golden", "gradient-128x64.bmp")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skip("updated golden")
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update-golden first): %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("BMP output differs from golden %s", goldenPath)
	}
}
