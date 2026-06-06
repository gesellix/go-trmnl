package admin

import (
	"net/http"
)

// DevicesList shows all registered devices.
func (h *Handler) DevicesList(w http.ResponseWriter, r *http.Request) {
	devices, err := h.store.ListDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "devices", map[string]any{"Nav": "devices", "Devices": devices})
}

// DeviceDetail shows one device with its telemetry and assignment form.
func (h *Handler) DeviceDetail(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d, err := h.store.GetDeviceByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	playlists, _ := h.store.ListPlaylists()

	// Preview thumbnail: the first visible screen of the assigned playlist, if
	// it has been rendered.
	var previewHash string
	if d.PlaylistID.Valid {
		if items, _ := h.store.ListPlaylistItems(d.PlaylistID.Int64); len(items) > 0 {
			if sc, err := h.store.GetScreen(items[0].ScreenID); err == nil && sc.RenderedHash.Valid {
				previewHash = sc.RenderedHash.String
			}
		}
	}

	h.render(w, "device", map[string]any{
		"Nav":         "devices",
		"Device":      d,
		"Playlists":   playlists,
		"PreviewHash": previewHash,
		"BaseURL":     h.baseURL,
	})
}

// DeviceUpdate saves the editable device fields.
func (h *Handler) DeviceUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := r.FormValue("name")
	refresh := atoiDefault(r.FormValue("refresh_rate"), 900)
	if refresh < 60 {
		refresh = 60
	}
	playlistID, _ := parseInt64(r.FormValue("playlist_id"))
	if err := h.store.UpdateDeviceSettings(id, name, refresh, nullInt(playlistID)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/devices/"+chiID(r), http.StatusFound)
}

// DeviceForceRefresh clears the cached render of every screen in the device's
// playlist so the next poll re-renders.
func (h *Handler) DeviceForceRefresh(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d, err := h.store.GetDeviceByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if d.PlaylistID.Valid {
		items, _ := h.store.ListPlaylistItems(d.PlaylistID.Int64)
		for _, it := range items {
			_ = h.store.ClearScreenRendered(it.ScreenID)
		}
	}
	http.Redirect(w, r, "/admin/devices/"+chiID(r), http.StatusFound)
}

// DeviceDelete removes a device.
func (h *Handler) DeviceDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.store.DeleteDevice(id)
	http.Redirect(w, r, "/admin/devices", http.StatusFound)
}

// DeviceLogs shows the most recent log entries for a device.
func (h *Handler) DeviceLogs(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d, err := h.store.GetDeviceByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	logs, _ := h.store.ListLogs(id, 200)
	h.render(w, "logs", map[string]any{"Nav": "devices", "Device": d, "Logs": logs})
}
