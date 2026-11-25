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

// placeholderStem is the file stem of the fallback image served when a device
// has no renderable screen yet. It exists in every format a device can be
// served (placeholder.bmp, placeholder.png).
const placeholderStem = "placeholder"

// Handler bundles the dependencies of the device endpoints.
type Handler struct {
	store       *store.Store
	baseURL     string
	uploadsDir  string
	assetsDir   string
	renderer    *render.Renderer
	disableAuth bool

	placeholderOnce sync.Once
	placeholderErr  error
}

// New creates a device API handler. baseURL is the public URL prefix used to
// build image URLs; uploadsDir is where rendered images live and are served
// from at /uploads.
func New(st *store.Store, baseURL, uploadsDir string, disableAuth bool) *Handler {
	return &Handler{
		store:       st,
		baseURL:     baseURL,
		uploadsDir:  uploadsDir,
		assetsDir:   filepath.Join(uploadsDir, "assets"),
		renderer:    render.NewRenderer(uploadsDir),
		disableAuth: disableAuth,
	}
}

// screenImage holds the served image's URL filename and its bare stem.
type screenImage struct {
	urlName string // e.g. "<hash>.bmp" or "placeholder.png"
	stem    string // e.g. "<hash>" or "placeholder"
}

// placeholderImage returns the fallback image in the device's format.
func (h *Handler) placeholderImage(d *store.Device) screenImage {
	_ = h.ensurePlaceholder()
	return screenImage{urlName: placeholderStem + d.ImageExt(), stem: placeholderStem}
}

// currentImage selects the device's next screen, renders it if needed, and
// returns the served image. It falls back to the placeholder when no screen is
// assigned or rendering fails.
func (h *Handler) currentImage(ctx context.Context, d *store.Device) screenImage {
	screen, err := playlist.NextScreen(h.store, d)
	if err != nil { // ErrNoScreen or a store error: fall back gracefully
		return h.placeholderImage(d)
	}
	hash, rerr := h.renderScreen(ctx, d, screen)
	if rerr != nil {
		return h.placeholderImage(d)
	}
	return screenImage{urlName: hash + d.ImageExt(), stem: hash}
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
	if hash, ok := h.cachedHash(d, screen, ttl); ok {
		return hash, nil
	}

	res, err := screens.Render(ctx, h.store, h.renderer, h.assetsDir, d, screen, h.ditherModeFor(screen))
	if err != nil {
		return "", err
	}
	return res.Hash, nil
}

// cachedHash returns the cached hash for this device's view of the screen, if
// it is still fresh and the file is present on disk. A device drawing the
// battery indicator or with a non-default panel size sees its own image, so it
// has its own cache entry (see screens.PerDeviceCache).
func (h *Handler) cachedHash(d *store.Device, screen *store.Screen, ttl time.Duration) (string, bool) {
	hash, renderedAt := "", int64(0)
	if screens.PerDeviceCache(d) {
		cachedState, ok, err := 0, false, error(nil)
		hash, renderedAt, cachedState, ok, err = h.store.DeviceScreenRender(d.ID, screen.ID)
		if err != nil || !ok {
			return "", false
		}
		// The indicator is part of the image, so a changed battery level has to
		// invalidate the cache even while the plugin's refresh interval holds.
		level, charging := screens.BatteryState(d)
		if d.ShowBattery && cachedState != render.BatteryState(level, charging) {
			return "", false
		}
	} else {
		if !screen.RenderedHash.Valid {
			return "", false
		}
		hash = screen.RenderedHash.String
		if screen.RenderedAt.Valid {
			renderedAt = screen.RenderedAt.Int64
		}
	}
	if hash == "" {
		return "", false
	}
	// The device fetches the file in its own format, so that one must exist.
	if _, err := os.Stat(filepath.Join(h.uploadsDir, hash+d.ImageExt())); err != nil {
		return "", false
	}
	if ttl > 0 && renderedAt > 0 {
		if time.Since(time.Unix(renderedAt, 0)) >= ttl {
			return "", false
		}
	}
	return hash, true
}

// ditherMode reads the global dithering mode, defaulting to Floyd-Steinberg.
func (h *Handler) ditherMode() render.Mode {
	if v, ok, _ := h.store.GetSetting("dither_mode"); ok {
		return render.ParseMode(v)
	}
	return render.FloydSteinberg
}

// ditherModeFor resolves the effective dithering mode for a screen: its
// per-screen override if set, otherwise the global default.
func (h *Handler) ditherModeFor(sc *store.Screen) render.Mode {
	if sc != nil && sc.DitherMode.Valid && sc.DitherMode.String != "" {
		return render.ParseMode(sc.DitherMode.String)
	}
	return h.ditherMode()
}

// Routes mounts the device endpoints onto r under /api.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.With(h.parseMAC).Get("/setup", h.Setup)
		r.With(h.loadDevice, h.requireToken).Get("/display", h.Display)
		r.With(h.loadDevice).Post("/log", h.Log)
	})
}

// ensurePlaceholder writes the fallback image to the uploads directory once,
// in every servable format. Files that already exist are kept, so an install
// that only has placeholder.bmp gains placeholder.png on the next start.
func (h *Handler) ensurePlaceholder() error {
	h.placeholderOnce.Do(func() {
		img := render.Placeholder([]string{
			"go-trmnl",
			"",
			"Device registered. No screen assigned yet.",
			"Add a screen in the admin UI.",
		})
		encoders := map[string]func(*bytes.Buffer) error{
			".bmp": func(buf *bytes.Buffer) error { return render.EncodeBMP1(buf, img) },
			".png": func(buf *bytes.Buffer) error { return render.EncodePNG1(buf, img) },
		}
		for ext, encode := range encoders {
			path := filepath.Join(h.uploadsDir, placeholderStem+ext)
			if _, err := os.Stat(path); err == nil {
				continue
			}
			var buf bytes.Buffer
			if err := encode(&buf); err != nil {
				h.placeholderErr = err
				return
			}
			if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
				h.placeholderErr = err
				return
			}
		}
	})
	return h.placeholderErr
}

func (h *Handler) imageURL(name string) string {
	return h.baseURL + "/uploads/" + name
}
