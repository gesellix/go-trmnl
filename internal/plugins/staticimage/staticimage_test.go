package staticimage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/gesellix/go-trmnl/internal/plugins"
)

// writePNG creates a small test PNG in dir and returns its filename.
func writePNG(t *testing.T, dir string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	name := "img.png"
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestDataModelAndRender(t *testing.T) {
	dir := t.TempDir()
	name := writePNG(t, dir, 200, 100)
	p := &Plugin{}
	in := plugins.RenderInput{
		Width:     800,
		Height:    480,
		AssetsDir: dir,
		Settings:  []byte(`{"file":"` + name + `"}`),
	}

	data, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatalf("DataModel: %v", err)
	}
	if _, ok := data.(image.Image); !ok {
		t.Fatalf("DataModel returned %T, want image.Image", data)
	}

	img, err := p.Render(context.Background(), in, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("bounds = %v, want 800x480", b)
	}
}

func TestDataModelMissingConfig(t *testing.T) {
	p := &Plugin{}
	if _, err := p.DataModel(context.Background(), plugins.RenderInput{Settings: []byte(`{}`)}); err == nil {
		t.Error("expected error when no file configured")
	}
}

func TestDataModelMissingFile(t *testing.T) {
	p := &Plugin{}
	in := plugins.RenderInput{AssetsDir: t.TempDir(), Settings: []byte(`{"file":"nope.png"}`)}
	if _, err := p.DataModel(context.Background(), in); err == nil {
		t.Error("expected error when file is absent")
	}
}

func TestRenderRejectsBadData(t *testing.T) {
	p := &Plugin{}
	if _, err := p.Render(context.Background(), plugins.RenderInput{Width: 800, Height: 480}, "not an image"); err == nil {
		t.Error("expected error for non-image data")
	}
}
