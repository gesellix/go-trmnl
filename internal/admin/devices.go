package admin

import (
	"log"
	"net/http"

	"github.com/gesellix/go-trmnl/internal/device"
	"github.com/gesellix/go-trmnl/internal/playlist"
	"github.com/gesellix/go-trmnl/internal/screens"
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

// DeviceCreate adds a new device by MAC.
func (h *Handler) DeviceCreate(w http.ResponseWriter, r *http.Request) {
	mac := r.FormValue("mac")
	if mac == "" {
		http.Error(w, "mac is required", http.StatusBadRequest)
		return
	}
	// Use the existing Provision logic to generate credentials.
	d, _, err := device.Provision(h.store, mac, "", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/devices/"+i64(d.ID), http.StatusFound)
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

	// Preview thumbnail: what this device was last served. A device drawing the
	// battery indicator has its own renders, which is the picture on its panel;
	// otherwise fall back to the first screen of the assigned playlist.
	var previewHash string
	if hash, ok, _ := h.store.LatestDeviceRender(d.ID); ok {
		previewHash = hash
	} else if d.PlaylistID.Valid {
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
	bundle := r.FormValue("font_bundle")
	showBattery := r.FormValue("show_battery") != ""
	if err := h.store.UpdateDeviceSettings(id, name, refresh, nullInt(playlistID), bundle, showBattery); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if fn := r.FormValue("special_function"); fn != "" {
		_ = h.store.SetSpecialFunction(id, fn)
	}
	http.Redirect(w, r, "/admin/devices/"+chiID(r), http.StatusFound)
}

// DeviceFirmware queues an OTA firmware update for the device's next poll.
func (h *Handler) DeviceFirmware(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	url := r.FormValue("firmware_url")
	if url == "" {
		http.Error(w, "firmware_url is required", http.StatusBadRequest)
		return
	}
	_ = h.store.QueueFirmwareUpdate(id, url)
	http.Redirect(w, r, "/admin/devices/"+chiID(r), http.StatusFound)
}

// DeviceFirmwareCancel cancels a pending firmware update.
func (h *Handler) DeviceFirmwareCancel(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.store.ClearFirmwareUpdate(id)
	http.Redirect(w, r, "/admin/devices/"+chiID(r), http.StatusFound)
}

// DeviceForceRefresh clears the cached render of every screen in the device's
// playlist and renders the screen the next poll will serve, so the device page
// shows the new picture right away.
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
	// Per-device renders are a separate cache; without this a device drawing
	// the battery indicator would keep serving its old image.
	_ = h.store.ClearDeviceRenders(d.ID)

	// Best-effort: on failure the next poll renders, as it did before.
	if sc, err := playlist.PeekScreen(h.store, d); err == nil {
		if _, err := screens.Render(r.Context(), h.store, h.renderer, h.assetsDir, d, sc, h.ditherModeFor(sc)); err != nil {
			log.Printf("admin: refresh render of screen %d for device %d: %v", sc.ID, d.ID, err)
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
