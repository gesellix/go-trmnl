package deviceapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gesellix/go-trmnl/internal/device"
	"github.com/gesellix/go-trmnl/internal/httpx"
	"github.com/gesellix/go-trmnl/internal/store"
)

// setupResponse is returned from GET /api/setup.
type setupResponse struct {
	Status     int    `json:"status"`
	APIKey     string `json:"api_key"`
	FriendlyID string `json:"friendly_id"`
	ImageURL   string `json:"image_url"`
	Filename   string `json:"filename"`
	Message    string `json:"message"`
}

// Setup auto-provisions an unknown device and returns its credentials.
func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	mac := macFromCtx(r.Context())
	d, created, err := device.Provision(h.store, mac, r.Header.Get("Model"), r.Header.Get("FW-Version"))
	if err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, "/problem#device_setup",
			"internal_server_error", "Failed to provision device.", r.URL.Path, nil)
		return
	}

	name, _ := h.ensurePlaceholder()
	msg := "Welcome to go-trmnl!"
	if !created {
		msg = "Device already registered."
	}
	httpx.WriteJSON(w, http.StatusOK, setupResponse{
		Status:     http.StatusOK,
		APIKey:     d.APIKey,
		FriendlyID: d.FriendlyID,
		ImageURL:   h.imageURL(name),
		Filename:   strings.TrimSuffix(name, ".bmp"),
		Message:    msg,
	})
}

// displayResponse is returned from GET /api/display. Field names match what the
// firmware parses; firmware_url is null when no update is pending.
type displayResponse struct {
	Status             int     `json:"status"`
	ImageURL           string  `json:"image_url"`
	Filename           string  `json:"filename"`
	RefreshRate        int     `json:"refresh_rate"`
	UpdateFirmware     bool    `json:"update_firmware"`
	FirmwareURL        *string `json:"firmware_url"`
	ResetFirmware      bool    `json:"reset_firmware"`
	SpecialFunction    string  `json:"special_function"`
	ImageURLTimeout    int     `json:"image_url_timeout"`
	TemperatureProfile string  `json:"temperature_profile"`
}

// Display persists telemetry from request headers and returns the screen to
// render. Until the render pipeline and playlists exist, it serves a
// placeholder image.
func (h *Handler) Display(w http.ResponseWriter, r *http.Request) {
	d := deviceFromCtx(r.Context())

	if err := h.store.UpdateTelemetry(d.ID, telemetryFromHeaders(r)); err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, "/problem#internal",
			"internal_server_error", "Failed to persist telemetry.", r.URL.Path, nil)
		return
	}

	img := h.currentImage(r.Context(), d)

	httpx.WriteJSON(w, http.StatusOK, displayResponse{
		Status:             0,
		ImageURL:           h.imageURL(img.urlName),
		Filename:           img.stem,
		RefreshRate:        d.RefreshRate,
		UpdateFirmware:     false,
		FirmwareURL:        nil,
		ResetFirmware:      false,
		SpecialFunction:    "none",
		ImageURLTimeout:    0,
		TemperatureProfile: "default",
	})
}

// logRequest is the body of POST /api/log.
type logRequest struct {
	Logs []logEntry `json:"logs"`
}

type logEntry struct {
	ID              *int64   `json:"id"`
	Message         *string  `json:"message"`
	CreatedAt       *int64   `json:"created_at"`
	WifiStatus      *string  `json:"wifi_status"`
	WifiSignal      *int64   `json:"wifi_signal"`
	SleepDuration   *int64   `json:"sleep_duration"`
	RefreshRate     *int64   `json:"refresh_rate"`
	FreeHeapSize    *int64   `json:"free_heap_size"`
	MaxAllocSize    *int64   `json:"max_alloc_size"`
	SourcePath      *string  `json:"source_path"`
	SourceLine      *int64   `json:"source_line"`
	WakeReason      *string  `json:"wake_reason"`
	FirmwareVersion *string  `json:"firmware_version"`
	BatteryVoltage  *float64 `json:"battery_voltage"`
	SpecialFunction *string  `json:"special_function"`
}

// Log stores a batch of device log entries and returns 204.
func (h *Handler) Log(w http.ResponseWriter, r *http.Request) {
	d := deviceFromCtx(r.Context())

	var req logRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, "/problem#log_payload",
			"unprocessable_content", "Invalid log payload.", r.URL.Path, nil)
		return
	}

	rows := make([]*store.DeviceLog, 0, len(req.Logs))
	for _, e := range req.Logs {
		rows = append(rows, e.toRow(d.ID))
	}
	if err := h.store.InsertLogs(rows); err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, "/problem#internal",
			"internal_server_error", "Failed to store logs.", r.URL.Path, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// telemetryFromHeaders extracts the device telemetry sent on the display poll.
func telemetryFromHeaders(r *http.Request) store.Telemetry {
	t := store.Telemetry{
		FWVersion:      nullStr(r.Header.Get("FW-Version")),
		Model:          nullStr(r.Header.Get("Model")),
		Width:          nullInt(r.Header.Get("Width")),
		Height:         nullInt(r.Header.Get("Height")),
		BatteryVoltage: nullFloat(r.Header.Get("Battery-Voltage")),
		RSSI:           nullInt(r.Header.Get("RSSI")),
		RefreshRate:    nullInt(r.Header.Get("Refresh-Rate")),
	}
	// The firmware reports charging state via either Battery-Charging or
	// USB-Connected depending on board.
	if v := firstNonEmpty(r.Header.Get("Battery-Charging"), r.Header.Get("USB-Connected")); v != "" {
		if b, ok := parseBool(v); ok {
			t.BatteryCharging = sql.NullBool{Bool: b, Valid: true}
		}
	}
	if v := r.Header.Get("WiFi-Status"); v != "" {
		t.WifiStatus = nullStr(v)
	}
	return t
}

func (e logEntry) toRow(deviceID int64) *store.DeviceLog {
	return &store.DeviceLog{
		DeviceID:        deviceID,
		LogID:           ptrInt(e.ID),
		Message:         ptrStr(e.Message),
		CreatedAt:       ptrInt(e.CreatedAt),
		WifiStatus:      ptrStr(e.WifiStatus),
		WifiSignal:      ptrInt(e.WifiSignal),
		SleepDuration:   ptrInt(e.SleepDuration),
		RefreshRate:     ptrInt(e.RefreshRate),
		FreeHeapSize:    ptrInt(e.FreeHeapSize),
		MaxAllocSize:    ptrInt(e.MaxAllocSize),
		SourcePath:      ptrStr(e.SourcePath),
		SourceLine:      ptrInt(e.SourceLine),
		WakeReason:      ptrStr(e.WakeReason),
		FirmwareVersion: ptrStr(e.FirmwareVersion),
		BatteryVoltage:  ptrFloat(e.BatteryVoltage),
		SpecialFunction: ptrStr(e.SpecialFunction),
	}
}

// --- small parsing/conversion helpers ---

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	return false, false
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt(s string) sql.NullInt64 {
	if s == "" {
		return sql.NullInt64{}
	}
	if v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return sql.NullInt64{Int64: v, Valid: true}
	}
	// Some numeric headers (e.g. Width) may arrive as floats; truncate.
	if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return sql.NullInt64{Int64: int64(f), Valid: true}
	}
	return sql.NullInt64{}
}

func nullFloat(s string) sql.NullFloat64 {
	if s == "" {
		return sql.NullFloat64{}
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return sql.NullFloat64{Float64: v, Valid: true}
	}
	return sql.NullFloat64{}
}

func ptrInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func ptrStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func ptrFloat(p *float64) sql.NullFloat64 {
	if p == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *p, Valid: true}
}
