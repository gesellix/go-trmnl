// Package admin implements the server's web admin UI: managing devices,
// screens, playlists, viewing logs, and editing settings. Templates and static
// assets are embedded for single-binary deployment.
package admin

import (
	"bytes"
	"database/sql"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gesellix/go-trmnl/internal/render"
	"github.com/gesellix/go-trmnl/internal/store"
	"github.com/go-chi/chi/v5"
)

// Handler holds the admin UI dependencies.
type Handler struct {
	store      *store.Store
	baseURL    string
	uploadsDir string
	assetsDir  string
	renderer   *render.Renderer
	tmpl       *templateSet
}

// New creates the admin handler.
func New(st *store.Store, baseURL, uploadsDir string) *Handler {
	return &Handler{
		store:      st,
		baseURL:    baseURL,
		uploadsDir: uploadsDir,
		assetsDir:  filepath.Join(uploadsDir, "assets"),
		renderer:   render.NewRenderer(uploadsDir),
		tmpl:       mustParseTemplates(),
	}
}

// Routes mounts the admin UI and its static assets onto r.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/admin", http.StatusFound)
	})
	r.Handle("/admin/static/*", http.StripPrefix("/admin/static/", staticFileServer()))

	r.Route("/admin", func(r chi.Router) {
		r.Get("/", h.Dashboard)

		r.Get("/devices", h.DevicesList)
		r.Get("/devices/{id}", h.DeviceDetail)
		r.Post("/devices/{id}", h.DeviceUpdate)
		r.Post("/devices/{id}/refresh", h.DeviceForceRefresh)
		r.Post("/devices/{id}/delete", h.DeviceDelete)
		r.Get("/devices/{id}/logs", h.DeviceLogs)

		r.Get("/screens", h.ScreensList)
		r.Post("/screens", h.ScreenCreate)
		r.Get("/screens/{id}", h.ScreenDetail)
		r.Post("/screens/{id}", h.ScreenUpdate)
		r.Post("/screens/{id}/preview", h.ScreenPreview)
		r.Post("/screens/{id}/upload", h.ScreenUpload)
		r.Post("/screens/{id}/delete", h.ScreenDelete)

		r.Get("/playlists", h.PlaylistsList)
		r.Post("/playlists", h.PlaylistCreate)
		r.Get("/playlists/{id}", h.PlaylistDetail)
		r.Post("/playlists/{id}/items", h.PlaylistAddItem)
		r.Post("/playlists/{id}/items/{itemID}/delete", h.PlaylistRemoveItem)
		r.Post("/playlists/{id}/delete", h.PlaylistDelete)

		r.Get("/settings", h.SettingsPage)
		r.Post("/settings", h.SettingsSave)
	})
}

// render executes the named page template wrapped in the layout.
func (h *Handler) render(w http.ResponseWriter, page string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	var buf bytes.Buffer
	if err := h.tmpl.execute(&buf, page, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func idParam(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, key), 10, 64)
}

// chiID returns the {id} URL param as a string, for building redirect paths.
func chiID(r *http.Request) string { return chi.URLParam(r, "id") }

func parseInt64(s string) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}

// i64 formats an int64 for use in redirect URLs.
func i64(v int64) string { return strconv.FormatInt(v, 10) }

// def returns v, or fallback when v is empty.
func def(fallback, v string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ditherMode reads the configured dithering mode, defaulting to Floyd-Steinberg.
func (h *Handler) ditherMode() render.Mode {
	if v, ok, _ := h.store.GetSetting("dither_mode"); ok {
		return render.ParseMode(v)
	}
	return render.FloydSteinberg
}

func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

// nullInt builds a valid NullInt64, or an invalid one when id <= 0.
func nullInt(id int64) sql.NullInt64 {
	if id <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: id, Valid: true}
}

func humanSince(unix int64) string {
	if unix == 0 {
		return "never"
	}
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h ago"
	default:
		return strconv.Itoa(int(d.Hours()/24)) + "d ago"
	}
}
