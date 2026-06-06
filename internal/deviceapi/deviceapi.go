// Package deviceapi implements the firmware-facing HTTP endpoints the TRMNL
// device polls: /api/setup, /api/display and /api/log. It is a thin layer over
// the device, store and render packages.
package deviceapi

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"

	"github.com/gesellix/go-trmnl/internal/render"
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

	placeholderOnce sync.Once
	placeholderErr  error
}

// New creates a device API handler. baseURL is the public URL prefix used to
// build image URLs; uploadsDir is where rendered images live and are served
// from at /uploads.
func New(st *store.Store, baseURL, uploadsDir string) *Handler {
	return &Handler{store: st, baseURL: baseURL, uploadsDir: uploadsDir}
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
