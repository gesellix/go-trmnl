package deviceapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gesellix/go-trmnl/internal/deviceapi"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"
)

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	r := server.New()
	deviceapi.New(st, "http://test.local", dir).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, st
}

func do(t *testing.T, ts *httptest.Server, method, path string, headers map[string]string, body string) *http.Response {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req, err := http.NewRequest(method, ts.URL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

const testMAC = "AA:BB:CC:DD:EE:FF"

func TestSetupProvisionsAndIsStable(t *testing.T) {
	ts, st := newTestServer(t)

	resp := do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": testMAC}, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup status = %d, want 200", resp.StatusCode)
	}
	var first struct {
		APIKey     string `json:"api_key"`
		FriendlyID string `json:"friendly_id"`
		ImageURL   string `json:"image_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&first); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if first.APIKey == "" || first.FriendlyID == "" {
		t.Fatalf("expected api_key and friendly_id, got %+v", first)
	}
	if !strings.HasPrefix(first.ImageURL, "http://test.local/uploads/") {
		t.Errorf("image_url = %q, want test.local/uploads prefix", first.ImageURL)
	}

	d, err := st.GetDeviceByMAC(testMAC)
	if err != nil {
		t.Fatalf("device not persisted: %v", err)
	}
	if d.APIKey != first.APIKey {
		t.Errorf("persisted api_key %q != response %q", d.APIKey, first.APIKey)
	}

	// Second setup returns the same credentials.
	resp2 := do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": testMAC}, "")
	defer resp2.Body.Close()
	var second struct {
		APIKey string `json:"api_key"`
	}
	json.NewDecoder(resp2.Body).Decode(&second)
	if second.APIKey != first.APIKey {
		t.Errorf("api_key changed on re-setup: %q -> %q", first.APIKey, second.APIKey)
	}
}

func TestSetupMissingID(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := do(t, ts, http.MethodGet, "/api/setup", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
}

func TestDisplay(t *testing.T) {
	ts, st := newTestServer(t)
	do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": testMAC}, "").Body.Close()
	d, _ := st.GetDeviceByMAC(testMAC)

	t.Run("good token", func(t *testing.T) {
		resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{
			"ID":              testMAC,
			"Access-Token":    d.APIKey,
			"Battery-Voltage": "3.74",
			"RSSI":            "-66",
			"FW-Version":      "1.5.2",
		}, "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var body struct {
			Status   int    `json:"status"`
			ImageURL string `json:"image_url"`
			Filename string `json:"filename"`
		}
		json.NewDecoder(resp.Body).Decode(&body)
		if body.Status != 0 {
			t.Errorf("status field = %d, want 0", body.Status)
		}
		if !strings.HasPrefix(body.ImageURL, "http://test.local/uploads/") {
			t.Errorf("image_url = %q", body.ImageURL)
		}
		if body.Filename == "" {
			t.Errorf("filename empty")
		}

		// Telemetry persisted.
		got, _ := st.GetDeviceByMAC(testMAC)
		if !got.BatteryVoltage.Valid || got.BatteryVoltage.Float64 != 3.74 {
			t.Errorf("battery_voltage not persisted: %+v", got.BatteryVoltage)
		}
		if !got.FWVersion.Valid || got.FWVersion.String != "1.5.2" {
			t.Errorf("fw_version not persisted: %+v", got.FWVersion)
		}
	})

	t.Run("bad token", func(t *testing.T) {
		resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{
			"ID": testMAC, "Access-Token": "wrong",
		}, "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("unknown device", func(t *testing.T) {
		resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{
			"ID": "11:22:33:44:55:66", "Access-Token": "x",
		}, "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDisplayFirmwareAndSpecialFunction(t *testing.T) {
	ts, st := newTestServer(t)
	do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": testMAC}, "").Body.Close()
	d, _ := st.GetDeviceByMAC(testMAC)

	if err := st.QueueFirmwareUpdate(d.ID, "http://test.local/fw/1.2.3.bin"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSpecialFunction(d.ID, "sleep"); err != nil {
		t.Fatal(err)
	}

	type body struct {
		UpdateFirmware  bool    `json:"update_firmware"`
		FirmwareURL     *string `json:"firmware_url"`
		SpecialFunction string  `json:"special_function"`
	}
	get := func() body {
		resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{"ID": testMAC, "Access-Token": d.APIKey}, "")
		defer resp.Body.Close()
		var b body
		json.NewDecoder(resp.Body).Decode(&b)
		return b
	}

	first := get()
	if !first.UpdateFirmware || first.FirmwareURL == nil || *first.FirmwareURL != "http://test.local/fw/1.2.3.bin" {
		t.Fatalf("first display: update not delivered: %+v", first)
	}
	if first.SpecialFunction != "sleep" {
		t.Errorf("special_function = %q, want sleep", first.SpecialFunction)
	}

	// One-shot: firmware update cleared on the next poll; special function persists.
	second := get()
	if second.UpdateFirmware {
		t.Errorf("firmware update should be one-shot, still set on second poll")
	}
	if second.SpecialFunction != "sleep" {
		t.Errorf("special_function should persist, got %q", second.SpecialFunction)
	}
}

func TestLog(t *testing.T) {
	ts, st := newTestServer(t)
	do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": testMAC}, "").Body.Close()
	d, _ := st.GetDeviceByMAC(testMAC)

	body := `{"logs":[
		{"id":1,"message":"boot","created_at":1700000000,"wifi_status":"connected","wifi_signal":-54,"battery_voltage":4.0,"wake_reason":"timer","firmware_version":"1.5.2","source_path":"src/bl.cpp","source_line":42,"refresh_rate":900,"sleep_duration":900,"free_heap_size":100000,"max_alloc_size":50000,"special_function":"none"}
	]}`
	resp := do(t, ts, http.MethodPost, "/api/log", map[string]string{
		"ID": testMAC, "Content-Type": "application/json",
	}, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	logs, err := st.ListLogs(d.ID, 10)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("got %d logs, want 1", len(logs))
	}
	if logs[0].Message.String != "boot" {
		t.Errorf("message = %q", logs[0].Message.String)
	}
}
