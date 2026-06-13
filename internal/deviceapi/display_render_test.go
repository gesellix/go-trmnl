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
	st.UpdateDeviceSettings(d.ID, "", 900, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic")

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
