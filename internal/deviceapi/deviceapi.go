// Package deviceapi implements the firmware-facing HTTP endpoints the TRMNL
// device polls: /api/setup, /api/display and /api/log. It is a thin layer over
// the device, store and render packages.
package deviceapi

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gesellix/go-trmnl/internal/playlist"
	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/render"
	"github.com/gesellix/go-trmnl/internal/screens"
	"github.com/gesellix/go-trmnl/internal/store"
	"github.com/go-chi/chi/v5"
)

// placeholderName is the filename of the fallback image served when a device
// has no renderable screen yet.
const placeholderName = "placeholder.bmp"

// Handler bundles the dependencies of the device endpoints.
type Handler struct {
	store      *store.Store
	baseURL    string
	uploadsDir string
	assetsDir  string
	renderer   *render.Renderer

	placeholderOnce sync.Once
	placeholderErr  error
}

// New creates a device API handler. baseURL is the public URL prefix used to
// build image URLs; uploadsDir is where rendered images live and are served
// from at /uploads.
func New(st *store.Store, baseURL, uploadsDir string) *Handler {
	return &Handler{
		store:      st,
		baseURL:    baseURL,
		uploadsDir: uploadsDir,
		assetsDir:  filepath.Join(uploadsDir, "assets"),
		renderer:   render.NewRenderer(uploadsDir),
	}
}

// screenImage holds the served image's URL filename and its bare stem.
type screenImage struct {
	urlName string // e.g. "<hash>.bmp" or "placeholder.bmp"
	stem    string // e.g. "<hash>" or "placeholder"
}

// currentImage selects the device's next screen, renders it if needed, and
// returns the served image. It falls back to the placeholder when no screen is
// assigned or rendering fails.
func (h *Handler) currentImage(ctx context.Context, d *store.Device) screenImage {
	screen, err := playlist.NextScreen(h.store, d)
	if err != nil { // ErrNoScreen or a store error: fall back gracefully
		name, _ := h.ensurePlaceholder()
		return screenImage{urlName: name, stem: trimExt(name)}
	}
	hash, rerr := h.renderScreen(ctx, d, screen)
	if rerr != nil {
		name, _ := h.ensurePlaceholder()
		return screenImage{urlName: name, stem: trimExt(name)}
	}
	return screenImage{urlName: hash + ".bmp", stem: hash}
}

// renderScreen returns the content hash of the screen's current image,
// rendering (and caching) it when the cache is stale or missing.
func (h *Handler) renderScreen(ctx context.Context, d *store.Device, screen *store.Screen) (string, error) {
	pluginRow, err := h.store.GetPlugin(screen.PluginID)
	if err != nil {
		return "", err
	}
	ttl := time.Duration(0)
	if p, ok := plugins.Get(pluginRow.Type); ok {
		ttl = p.DefaultRefresh()
	}
	if hash, ok := h.cachedHash(screen, ttl); ok {
		return hash, nil
	}

	res, err := screens.Render(ctx, h.store, h.renderer, h.assetsDir, d, screen, h.ditherMode())
	if err != nil {
		return "", err
	}
	return res.Hash, nil
}

// cachedHash returns the screen's cached hash if it is still fresh and the file
// is present on disk.
func (h *Handler) cachedHash(screen *store.Screen, ttl time.Duration) (string, bool) {
	if !screen.RenderedHash.Valid {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(h.uploadsDir, screen.RenderedHash.String+".bmp")); err != nil {
		return "", false
	}
	if ttl > 0 && screen.RenderedAt.Valid {
		if time.Since(time.Unix(screen.RenderedAt.Int64, 0)) >= ttl {
			return "", false
		}
	}
	return screen.RenderedHash.String, true
}

// ditherMode reads the configured dithering mode, defaulting to Floyd-Steinberg.
func (h *Handler) ditherMode() render.Mode {
	if v, ok, _ := h.store.GetSetting("dither_mode"); ok {
		return render.ParseMode(v)
	}
	return render.FloydSteinberg
}

func trimExt(name string) string {
	if i := len(name) - len(filepath.Ext(name)); i >= 0 {
		return name[:i]
	}
	return name
}

// Routes mounts the device endpoints onto r under /api.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.With(h.parseMAC).Get("/setup", h.Setup)
		r.With(h.loadDevice, h.requireToken).Get("/display", h.Display)
		r.With(h.loadDevice).Post("/log", h.Log)
	})
}

// ensurePlaceholder writes the fallback image to the uploads directory once.
func (h *Handler) ensurePlaceholder() (string, error) {
	h.placeholderOnce.Do(func() {
		path := filepath.Join(h.uploadsDir, placeholderName)
		if _, err := os.Stat(path); err == nil {
			return
		}
		img := render.Placeholder([]string{
			"go-trmnl",
			"",
			"Device registered. No screen assigned yet.",
			"Add a screen in the admin UI.",
		})
		var buf bytes.Buffer
		if err := render.EncodeBMP1(&buf, img); err != nil {
			h.placeholderErr = err
			return
		}
		h.placeholderErr = os.WriteFile(path, buf.Bytes(), 0o644)
	})
	return placeholderName, h.placeholderErr
}

func (h *Handler) imageURL(name string) string {
	return h.baseURL + "/uploads/" + name
}
