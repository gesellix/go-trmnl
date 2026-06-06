package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/screens"
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
	h.render(w, "screen", map[string]any{
		"Nav":        "screens",
		"Screen":     sc,
		"PluginType": pluginType,
		"IsImage":    pluginType == "staticimage",
		"BaseURL":    h.baseURL,
	})
}

// ScreenUpdate saves a screen's name and settings JSON.
func (h *Handler) ScreenUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := r.FormValue("name")
	settings := r.FormValue("settings_json")
	if !json.Valid([]byte(settings)) {
		http.Error(w, "settings is not valid JSON", http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateScreenSettings(id, name, settings); err != nil {
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
	if _, rerr := screens.Render(r.Context(), h.store, h.renderer, h.assetsDir, nil, sc, h.ditherMode()); rerr != nil {
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
	defer file.Close()

	if err := os.MkdirAll(h.assetsDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	name := randomName() + filepath.Ext(hdr.Filename)
	dst, err := os.Create(filepath.Join(h.assetsDir, name))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dst.Close()

	settings, _ := json.Marshal(map[string]string{"file": name})
	if err := h.store.UpdateScreenSettings(id, sc.Name, string(settings)); err != nil {
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

func randomName() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
