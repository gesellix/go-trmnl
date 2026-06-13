package admin

import (
	"net/http"
)

// Dashboard shows summary counts and recently-seen devices.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	devices, _ := h.store.ListDevices()
	scrs, _ := h.store.ListScreens()
	pls, _ := h.store.ListPlaylists()

	recent := devices
	if len(recent) > 8 {
		recent = recent[:8]
	}
	h.render(w, "dashboard", map[string]any{
		"Nav":           "dashboard",
		"DeviceCount":   len(devices),
		"ScreenCount":   len(scrs),
		"PlaylistCount": len(pls),
		"Devices":       recent,
		"BaseURL":       h.baseURL,
	})
}

// SettingsPage renders the settings form.
func (h *Handler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	mode, _, _ := h.store.GetSetting("dither_mode")
	if mode == "" {
		mode = "floyd_steinberg"
	}
	fontSans, _, _ := h.store.GetSetting("font_sans")
	fontMono, _, _ := h.store.GetSetting("font_mono")
	fontTitle, _, _ := h.store.GetSetting("font_title")

	h.render(w, "settings", map[string]any{
		"Nav":        "settings",
		"DitherMode": mode,
		"FontSans":   fontSans,
		"FontMono":   fontMono,
		"FontTitle":  fontTitle,
		"BaseURL":    h.baseURL,
	})
}

// SettingsSave persists settings. The dither_mode value is stored as the token
// render.ParseMode understands ("threshold" or "floyd_steinberg").
func (h *Handler) SettingsSave(w http.ResponseWriter, r *http.Request) {
	mode := "floyd_steinberg"
	if r.FormValue("dither_mode") == "threshold" {
		mode = "threshold"
	}
	_ = h.store.SetSetting("dither_mode", mode)
	_ = h.store.SetSetting("font_sans", r.FormValue("font_sans"))
	_ = h.store.SetSetting("font_mono", r.FormValue("font_mono"))
	_ = h.store.SetSetting("font_title", r.FormValue("font_title"))

	http.Redirect(w, r, "/admin/settings", http.StatusFound)
}
