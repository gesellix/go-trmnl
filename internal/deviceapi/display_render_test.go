package deviceapi_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gesellix/go-trmnl/internal/device"
	"github.com/gesellix/go-trmnl/internal/deviceapi"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"

	_ "github.com/gesellix/go-trmnl/internal/plugins/clock"
)

func newTestServerDir(t *testing.T) (*httptest.Server, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	r := server.New()
	deviceapi.New(st, "http://test.local", dir, false).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, st, dir
}

func TestDisplayRendersClockScreen(t *testing.T) {
	ts, st, dir := newTestServerDir(t)

	// Provision a device and give it a playlist with one clock screen.
	device.Provision(st, testMAC, "", "")
	do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": testMAC}, "").Body.Close()
	d, _ := st.GetDeviceByMAC(testMAC)
	pl, _ := st.CreatePlaylist("default")
	pg, _ := st.CreatePlugin("clock", "Clock")
	sc, _ := st.CreateScreen(pg.ID, "Clock", `{"use_24h":true,"label":"Office"}`)
	st.AddPlaylistItem(pl.ID, sc.ID)
	st.UpdateDeviceSettings(d.ID, "", 900, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic", false)

	resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{
		"ID": testMAC, "Access-Token": d.APIKey,
	}, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		ImageURL string `json:"image_url"`
		Filename string `json:"filename"`
	}
	json.NewDecoder(resp.Body).Decode(&body)

	if body.Filename == "placeholder" || body.Filename == "" {
		t.Fatalf("expected a rendered hash filename, got %q", body.Filename)
	}
	if !strings.HasSuffix(body.ImageURL, body.Filename+".bmp") {
		t.Errorf("image_url %q does not match filename %q", body.ImageURL, body.Filename)
	}

	// The rendered BMP and PNG must exist on disk.
	for _, ext := range []string{".bmp", ".png"} {
		if _, err := os.Stat(filepath.Join(dir, body.Filename+ext)); err != nil {
			t.Errorf("rendered file %s missing: %v", body.Filename+ext, err)
		}
	}

	// The screen's rendered hash is recorded.
	got, _ := st.GetScreen(sc.ID)
	if !got.RenderedHash.Valid || got.RenderedHash.String != body.Filename {
		t.Errorf("screen rendered_hash = %+v, want %q", got.RenderedHash, body.Filename)
	}
}

// provisionDevice registers a device, assigns it the playlist and sets whether
// it draws the battery indicator.
func provisionDevice(t *testing.T, ts *httptest.Server, st *store.Store, mac string, playlistID int64, battery bool) *store.Device {
	t.Helper()
	if _, _, err := device.Provision(st, mac, "", ""); err != nil {
		t.Fatalf("provision %s: %v", mac, err)
	}
	do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": mac}, "").Body.Close()
	d, err := st.GetDeviceByMAC(mac)
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if err := st.UpdateDeviceSettings(d.ID, "", 900, sql.NullInt64{Int64: playlistID, Valid: true}, "classic", battery); err != nil {
		t.Fatalf("settings: %v", err)
	}
	d, _ = st.GetDeviceByMAC(mac)
	return d
}

// displayFilename polls /api/display with a battery voltage and returns the
// served image's content hash.
func displayFilename(t *testing.T, ts *httptest.Server, d *store.Device, volts string) string {
	t.Helper()
	resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{
		"ID": d.MAC, "Access-Token": d.APIKey, "Battery-Voltage": volts,
	}, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Filename == "" || body.Filename == "placeholder" {
		t.Fatalf("no screen rendered: filename = %q", body.Filename)
	}
	return body.Filename
}

// A device with the battery indicator sees a different image than one without,
// so the two must not share the per-screen render cache.
func TestBatteryIndicatorRendersPerDevice(t *testing.T) {
	ts, st, _ := newTestServerDir(t)
	pl, _ := st.CreatePlaylist("default")
	pg, _ := st.CreatePlugin("clock", "Clock")
	sc, _ := st.CreateScreen(pg.ID, "Clock", `{"use_24h":true,"label":"Office"}`)
	st.AddPlaylistItem(pl.ID, sc.ID)

	plain := provisionDevice(t, ts, st, "AA:BB:CC:DD:EE:21", pl.ID, false)
	withBattery := provisionDevice(t, ts, st, "AA:BB:CC:DD:EE:22", pl.ID, true)

	plainName := displayFilename(t, ts, plain, "3.95")
	batteryName := displayFilename(t, ts, withBattery, "3.95")
	if plainName == batteryName {
		t.Fatalf("both devices got the same image %q; the indicator was not drawn or the cache is shared", plainName)
	}

	// Serving the same device again must hit its own cache.
	if again := displayFilename(t, ts, withBattery, "3.95"); again != batteryName {
		t.Errorf("second poll re-rendered: %q != %q", again, batteryName)
	}

	// A voltage within the same level keeps the image; a lower level changes it.
	if same := displayFilename(t, ts, withBattery, "4.05"); same != batteryName {
		t.Errorf("4.05 V changed the image although it is the same level: %q != %q", same, batteryName)
	}
	if lower := displayFilename(t, ts, withBattery, "3.35"); lower == batteryName {
		t.Error("an empty battery drew the same image as a full one")
	}
}
