package admin

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/screens"
	"github.com/gesellix/go-trmnl/internal/store"
)

// ScreensList shows all screens and the create form.
func (h *Handler) ScreensList(w http.ResponseWriter, r *http.Request) {
	scrs, _ := h.store.ListScreens()
	type row struct {
		ID         int64
		Name       string
		PluginType string
		Rendered   string
	}
	rows := make([]row, 0, len(scrs))
	for _, sc := range scrs {
		pt := ""
		if pg, err := h.store.GetPlugin(sc.PluginID); err == nil {
			pt = pg.Type
		}
		rendered := ""
		if sc.RenderedHash.Valid {
			rendered = sc.RenderedHash.String
		}
		rows = append(rows, row{ID: sc.ID, Name: sc.Name, PluginType: pt, Rendered: rendered})
	}
	h.render(w, "screens", map[string]any{
		"Nav":     "screens",
		"Screens": rows,
		"Plugins": plugins.All(),
		"BaseURL": h.baseURL,
	})
}

// ScreenCreate creates a plugin instance + screen.
func (h *Handler) ScreenCreate(w http.ResponseWriter, r *http.Request) {
	pluginType := r.FormValue("plugin_type")
	name := def(pluginType, r.FormValue("name"))
	if _, ok := plugins.Get(pluginType); !ok {
		http.Error(w, "unknown plugin type", http.StatusBadRequest)
		return
	}
	pg, err := h.store.CreatePlugin(pluginType, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sc, err := h.store.CreateScreen(pg.ID, name, "{}")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/screens/"+i64(sc.ID), http.StatusFound)
}

// ScreenDetail shows a screen's settings editor and preview.
func (h *Handler) ScreenDetail(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sc, err := h.store.GetScreen(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pg, _ := h.store.GetPlugin(sc.PluginID)
	pluginType := ""
	if pg != nil {
		pluginType = pg.Type
	}
	globalDither := "floyd_steinberg"
	if v, ok, _ := h.store.GetSetting("dither_mode"); ok && v != "" {
		globalDither = v
	}
	dither := ""
	if sc.DitherMode.Valid {
		dither = sc.DitherMode.String
	}
	devices, _ := h.store.ListDevices()
	data := map[string]any{
		"Nav":              "screens",
		"Screen":           sc,
		"PluginType":       pluginType,
		"IsImage":          pluginType == "staticimage",
		"IsFamilyCalendar": pluginType == "familycalendar",
		"DitherMode":       dither,
		"GlobalDither":     globalDither,
		"BaseURL":          h.baseURL,
		"Devices":          devices,
	}
	if pluginType == "familycalendar" {
		h.addFamilyCalendarSettings(data, sc.SettingsJSON)
	}
	h.render(w, "screen", data)
}

// addFamilyCalendarSettings populates the friendly familycalendar settings form
// data: the list of calendar accounts (with the screen's current selection) and
// the scalar options parsed from the screen's settings JSON.
func (h *Handler) addFamilyCalendarSettings(data map[string]any, settingsJSON string) {
	var fc struct {
		Accounts  []int64 `json:"accounts"`
		Days      int     `json:"days"`
		MaxEvents int     `json:"max_events"`
		Label     string  `json:"label"`
		Use24h    bool    `json:"use_24h"`

		Location  string  `json:"location"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Units     string  `json:"units"`
	}
	_ = json.Unmarshal([]byte(settingsJSON), &fc)
	selected := make(map[int64]bool, len(fc.Accounts))
	for _, id := range fc.Accounts {
		selected[id] = true
	}
	type accRow struct {
		ID       int64
		Name     string
		Provider string
		Marker   string
		Selected bool
	}
	var rows []accRow
	if h.cal != nil {
		accs, _ := h.cal.Accounts()
		for _, a := range accs {
			rows = append(rows, accRow{ID: a.ID, Name: a.Name, Provider: string(a.Provider), Marker: a.Marker, Selected: selected[a.ID]})
		}
	}
	days := fc.Days
	if days <= 0 {
		days = 14
	}
	maxEvents := fc.MaxEvents
	if maxEvents <= 0 {
		maxEvents = 30
	}
	data["FCAccounts"] = rows
	data["FCDays"] = days
	data["FCMaxEvents"] = maxEvents
	data["FCLabel"] = fc.Label
	data["FCUse24h"] = fc.Use24h

	data["FCLocation"] = fc.Location
	data["FCLatitude"] = fc.Latitude
	data["FCLongitude"] = fc.Longitude
	data["FCUnits"] = fc.Units
}

// ScreenUpdate saves a screen's name and settings JSON.
func (h *Handler) ScreenUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := r.FormValue("name")

	// The familycalendar screen uses a friendly form (account checkboxes + a few
	// options) instead of the raw JSON editor; compose its settings from those
	// fields. Other plugins use the JSON textarea.
	var settings string
	if sc, gerr := h.store.GetScreen(id); gerr == nil {
		if pg, perr := h.store.GetPlugin(sc.PluginID); perr == nil && pg.Type == "familycalendar" {
			settings = familyCalendarSettings(r)
		}
	}
	if settings == "" {
		settings = r.FormValue("settings_json")
		if !json.Valid([]byte(settings)) {
			http.Error(w, "settings is not valid JSON", http.StatusBadRequest)
			return
		}
	}
	if err := h.store.UpdateScreenSettings(id, name, settings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.store.UpdateScreenDither(id, parseDitherOverride(r.FormValue("dither_mode"))); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/screens/"+chiID(r), http.StatusFound)
}

// ScreenPreview renders the screen now so the detail page can show it.
func (h *Handler) ScreenPreview(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sc, err := h.store.GetScreen(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var d *store.Device
	if devID, ok := parseInt64(r.FormValue("device_id")); ok {
		d, _ = h.store.GetDeviceByID(devID)
	}
	if _, rerr := screens.Render(r.Context(), h.store, h.renderer, h.assetsDir, d, sc, h.ditherModeFor(sc)); rerr != nil {
		http.Error(w, "render failed: "+rerr.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/screens/"+chiID(r), http.StatusFound)
}

// ScreenUpload stores an uploaded image asset and points the (staticimage)
// screen's settings at it.
func (h *Handler) ScreenUpload(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sc, err := h.store.GetScreen(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file, hdr, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "no file uploaded", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	if err = os.MkdirAll(h.assetsDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	name := randomName() + filepath.Ext(hdr.Filename)
	dst, err := os.Create(filepath.Join(h.assetsDir, name))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err = io.Copy(dst, file); err != nil {
		_ = dst.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = dst.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	settings, err := json.Marshal(map[string]string{"file": name})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = h.store.UpdateScreenSettings(id, sc.Name, string(settings)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/screens/"+chiID(r), http.StatusFound)
}

// ScreenDelete removes a screen (and its plugin instance).
func (h *Handler) ScreenDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.store.DeleteScreen(id)
	http.Redirect(w, r, "/admin/screens", http.StatusFound)
}

// parseDitherOverride maps a screen-editor form value to a per-screen dither
// override. An empty value (or unknown token) means "inherit the global
// setting" and is stored as NULL.
func parseDitherOverride(v string) sql.NullString {
	if v == "threshold" || v == "floyd_steinberg" {
		return sql.NullString{String: v, Valid: true}
	}
	return sql.NullString{}
}

// familyCalendarSettings builds the familycalendar settings JSON from the
// friendly form fields (account checkboxes + options). No accounts selected
// means "all accounts" (the plugin's default), so the key is omitted.
func familyCalendarSettings(r *http.Request) string {
	_ = r.ParseForm()
	var accounts []int64
	for _, s := range r.Form["accounts"] {
		if v, ok := parseInt64(s); ok {
			accounts = append(accounts, v)
		}
	}
	m := map[string]any{}
	if len(accounts) > 0 {
		m["accounts"] = accounts
	}
	if v, ok := parseInt64(r.FormValue("days")); ok && v > 0 {
		m["days"] = v
	}
	if v, ok := parseInt64(r.FormValue("max_events")); ok && v > 0 {
		m["max_events"] = v
	}
	if l := r.FormValue("label"); l != "" {
		m["label"] = l
	}
	if r.FormValue("use_24h") != "" {
		m["use_24h"] = true
	}
	if v := r.FormValue("location"); v != "" {
		m["location"] = v
	}
	if v, err := strconv.ParseFloat(r.FormValue("latitude"), 64); err == nil && v != 0 {
		m["latitude"] = v
	}
	if v, err := strconv.ParseFloat(r.FormValue("longitude"), 64); err == nil && v != 0 {
		m["longitude"] = v
	}
	if v := r.FormValue("units"); v != "" {
		m["units"] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func randomName() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
