// Package admin implements the server's web admin UI: managing devices,
// screens, playlists, viewing logs, and editing settings. Templates and static
// assets are embedded for single-binary deployment.
package admin

import (
	"bytes"
	"crypto/subtle"
	"database/sql"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gesellix/go-trmnl/internal/calendar"
	"github.com/gesellix/go-trmnl/internal/render"
	"github.com/gesellix/go-trmnl/internal/store"
	"github.com/go-chi/chi/v5"
)

// Auth holds optional HTTP Basic Auth credentials for the admin UI. When
// Password is empty, authentication is disabled.
type Auth struct {
	User     string
	Password string
}

// Handler holds the admin UI dependencies.
type Handler struct {
	store      *store.Store
	baseURL    string
	uploadsDir string
	assetsDir  string
	renderer   *render.Renderer
	tmpl       *templateSet
	auth       Auth
	cal        *calendar.Service
}

// New creates the admin handler. auth guards the UI when its Password is set.
// cal powers the family calendar account pages; it may be nil.
func New(st *store.Store, baseURL, uploadsDir string, auth Auth, cal *calendar.Service) *Handler {
	return &Handler{
		store:      st,
		baseURL:    baseURL,
		uploadsDir: uploadsDir,
		assetsDir:  filepath.Join(uploadsDir, "assets"),
		renderer:   render.NewRenderer(uploadsDir),
		tmpl:       mustParseTemplates(),
		auth:       auth,
		cal:        cal,
	}
}

// requireAuth enforces HTTP Basic Auth when a password is configured.
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.auth.Password == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(user), []byte(h.auth.User)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(h.auth.Password)) == 1
		if !ok || !userOK || !passOK {
			w.Header().Set("WWW-Authenticate", `Basic realm="go-trmnl admin", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Routes mounts the admin UI and its static assets onto r.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/admin", http.StatusFound)
	})
	r.With(h.requireAuth).Handle("/admin/static/*", http.StripPrefix("/admin/static/", staticFileServer()))

	r.Route("/admin", func(r chi.Router) {
		r.Use(h.requireAuth)
		r.Get("/", h.Dashboard)

		r.Get("/devices", h.DevicesList)
		r.Post("/devices", h.DeviceCreate)
		r.Get("/devices/{id}", h.DeviceDetail)
		r.Post("/devices/{id}", h.DeviceUpdate)
		r.Post("/devices/{id}/refresh", h.DeviceForceRefresh)
		r.Post("/devices/{id}/firmware", h.DeviceFirmware)
		r.Post("/devices/{id}/firmware/cancel", h.DeviceFirmwareCancel)
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

		r.Get("/calendar", h.CalendarList)
		r.Post("/calendar/oauth-clients", h.CalendarOAuthClientCreate)
		r.Post("/calendar/oauth-clients/{id}/delete", h.CalendarOAuthClientDelete)
		r.Get("/calendar/google/start", h.CalendarGoogleStart)
		r.Get("/oauth/google/callback", h.CalendarGoogleCallback)
		r.Post("/calendar/caldav", h.CalendarCalDAVCreate)
		r.Get("/calendar/{id}", h.CalendarAccountDetail)
		r.Post("/calendar/{id}", h.CalendarAccountUpdate)
		r.Post("/calendar/{id}/sync", h.CalendarAccountSync)
		r.Post("/calendar/{id}/delete", h.CalendarAccountDelete)
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
