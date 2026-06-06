// Package staticimage is a built-in plugin that displays a user-uploaded image,
// scaled to fit the panel. The dithering/1-bit conversion happens later in the
// render pipeline.
package staticimage

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"time"

	_ "image/gif"  // register GIF decoder
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder

	xdraw "golang.org/x/image/draw"

	"github.com/gesellix/go-trmnl/internal/plugins"
	_ "golang.org/x/image/bmp"  // register BMP decoder
	_ "golang.org/x/image/webp" // register WebP decoder
)

func init() { plugins.Register(&Plugin{}) }

// Plugin renders a static uploaded image.
type Plugin struct{}

// Type returns the registry key.
func (p *Plugin) Type() string { return "staticimage" }

// Title returns the human-friendly plugin name.
func (p *Plugin) Title() string { return "Static Image" }

// DefaultRefresh returns the cache TTL hint; static images change rarely.
func (p *Plugin) DefaultRefresh() time.Duration { return 24 * time.Hour }

// settings holds the asset filename (relative to the assets dir) to display.
type settings struct {
	File string `json:"file"`
}

// DataModel loads and decodes the configured image file.
func (p *Plugin) DataModel(_ context.Context, in plugins.RenderInput) (any, error) {
	var s settings
	if len(in.Settings) > 0 {
		_ = json.Unmarshal(in.Settings, &s)
	}
	if s.File == "" {
		return nil, fmt.Errorf("staticimage: no file configured")
	}
	path := filepath.Join(in.AssetsDir, filepath.Base(s.File))
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("staticimage: open %s: %w", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("staticimage: decode %s: %w", path, err)
	}
	return img, nil
}

// Render scales the decoded image to fit the panel, letterboxed on white.
func (p *Plugin) Render(_ context.Context, in plugins.RenderInput, raw any) (*image.RGBA, error) {
	src, ok := raw.(image.Image)
	if !ok || src == nil {
		return nil, fmt.Errorf("staticimage: no image data")
	}
	dst := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	// Scale to fit while preserving aspect ratio (letterboxed on white).
	sb := src.Bounds()
	scale := min(
		float64(in.Width)/float64(sb.Dx()),
		float64(in.Height)/float64(sb.Dy()),
	)
	dw, dh := int(float64(sb.Dx())*scale), int(float64(sb.Dy())*scale)
	ox, oy := (in.Width-dw)/2, (in.Height-dh)/2
	rect := image.Rect(ox, oy, ox+dw, oy+dh)
	xdraw.CatmullRom.Scale(dst, rect, src, sb, xdraw.Over, nil)
	return dst, nil
}
