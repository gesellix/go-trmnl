package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

// FontStyle defines the abstract font family to be used for rendering.
type FontStyle int

const (
	// StyleSans is the standard sans-serif font family.
	StyleSans FontStyle = iota
	// StyleMono is the monospace font family.
	StyleMono
	// StyleTitle is the font family used for titles and headings (usually bold).
	StyleTitle
)

// Embedded Go fonts, parsed once and shared (read-only) across every FontSet
// that falls back to the defaults. Parsing them per render would be wasteful.
var (
	goFontsOnce             sync.Once
	goSans, goMono, goTitle *opentype.Font
	goFontsErr              error
)

func loadGoFonts() {
	goFontsOnce.Do(func() {
		goSans, goFontsErr = opentype.Parse(goregular.TTF)
		if goFontsErr != nil {
			return
		}
		goTitle, _ = opentype.Parse(gobold.TTF)
		goMono, _ = opentype.Parse(gomono.TTF)
	})
}

// FontSet is a self-contained set of resolved fonts plus its own face cache.
// Each render builds its own FontSet (see NewFontSet), so concurrent renders
// using different bundles or overrides never share or clobber font state.
type FontSet struct {
	mu        sync.Mutex
	fonts     map[FontStyle]*opentype.Font
	faceCache map[string]font.Face
}

// NewFontSet resolves the fonts for one render using the priority hierarchy
// custom overrides > font bundle > built-in Go fonts. assetsDir is the uploads
// assets directory; bundle is "classic" or "trmnl"; sans/mono/title are
// optional filenames (relative to assetsDir or absolute) overriding each style.
func NewFontSet(assetsDir, bundle, sans, mono, title string) *FontSet {
	loadGoFonts()

	fs := &FontSet{
		fonts:     make(map[FontStyle]*opentype.Font, 3),
		faceCache: make(map[string]font.Face),
	}

	load := func(path string) *opentype.Font {
		if path == "" {
			return nil
		}
		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(assetsDir, path)
		}
		b, err := os.ReadFile(fullPath)
		if err != nil {
			return nil
		}
		f, err := opentype.Parse(b)
		if err != nil {
			return nil
		}
		return f
	}

	// 1. User overrides.
	fSans := load(sans)
	fMono := load(mono)
	fTitle := load(title)

	// 2. Bundle assets, expected under assetsDir/bundles/{classic,trmnl}/.
	if fSans == nil {
		if bundle == "classic" {
			fSans = load(filepath.Join("bundles", bundle, "Inter.ttf"))
		} else {
			fSans = load(filepath.Join("bundles", bundle, "TRMNL21-Regular.ttf"))
		}
	}
	if fMono == nil {
		if bundle == "classic" {
			fMono = load(filepath.Join("bundles", bundle, "Inter.ttf")) // Mono fallback for classic
		} else {
			fMono = load(filepath.Join("bundles", bundle, "TRMNL21-Regular.ttf"))
		}
	}
	if fTitle == nil {
		if bundle == "classic" {
			fTitle = load(filepath.Join("bundles", bundle, "BlockKie.ttf"))
		} else {
			fTitle = load(filepath.Join("bundles", bundle, "TRMNL21-Bold.ttf"))
		}
	}

	// 3. Fall back to the shared built-in Go fonts (no re-parse).
	fs.fonts[StyleSans] = orGo(fSans, goSans)
	fs.fonts[StyleMono] = orGo(fMono, goMono)
	fs.fonts[StyleTitle] = orGo(fTitle, goTitle)

	return fs
}

func orGo(f, fallback *opentype.Font) *opentype.Font {
	if f != nil {
		return f
	}
	return fallback
}

// Face returns a cached font face for the given style and point size. Faces are
// safe to reuse across renders that share this FontSet. A nil FontSet falls
// back to the package default (built-in Go fonts), so draw-only unit tests and
// nil-device previews keep working.
func (fs *FontSet) Face(points float64, style FontStyle) (font.Face, error) {
	if fs == nil {
		return defaultFontSet().Face(points, style)
	}
	loadGoFonts()
	if goFontsErr != nil {
		return nil, goFontsErr
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	key := fmt.Sprintf("%v-%d", points, style)
	if f, ok := fs.faceCache[key]; ok {
		return f, nil
	}

	src := fs.fonts[style]
	if src == nil {
		src = fs.fonts[StyleSans]
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{
		Size:    points,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	fs.faceCache[key] = f
	return f, nil
}

var (
	defaultSetOnce sync.Once
	defaultSet     *FontSet
)

// defaultFontSet is the package-level set backed by the built-in Go fonts.
func defaultFontSet() *FontSet {
	defaultSetOnce.Do(func() {
		defaultSet = NewFontSet("", "", "", "", "")
	})
	return defaultSet
}

// Face returns a cached face from the built-in Go fonts. Kept for callers that
// don't carry a per-render FontSet.
func Face(points float64, bold bool) (font.Face, error) {
	style := StyleSans
	if bold {
		style = StyleTitle
	}
	return FaceStyle(points, style)
}

// FaceStyle returns a cached face for a style from the built-in Go fonts.
func FaceStyle(points float64, style FontStyle) (font.Face, error) {
	return defaultFontSet().Face(points, style)
}
