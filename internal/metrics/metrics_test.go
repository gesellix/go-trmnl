package metrics_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gesellix/go-trmnl/internal/metrics"
	"github.com/gesellix/go-trmnl/internal/store"
)

func openTest(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func scrape(t *testing.T, h *metrics.Handler, user, pass string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	rec := httptest.NewRecorder()
	h.Mux().ServeHTTP(rec, req)
	return rec
}

func TestScrapeReportsDeviceTelemetry(t *testing.T) {
	st := openTest(t)
	d, err := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:01", APIKey: "k", FriendlyID: "ABC123"})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err = st.UpdateTelemetry(d.ID, store.Telemetry{
		BatteryVoltage:  sql.NullFloat64{Float64: 3.95, Valid: true},
		BatteryCharging: sql.NullBool{Bool: true, Valid: true},
		RSSI:            sql.NullInt64{Int64: -62, Valid: true},
		Model:           sql.NullString{String: "og_plus", Valid: true},
	}); err != nil {
		t.Fatalf("telemetry: %v", err)
	}

	body := scrape(t, metrics.New(st, "v1.2.3", metrics.Auth{}), "", "").Body.String()
	for _, want := range []string{
		`trmnl_build_info{version="v1.2.3"} 1`,
		`trmnl_devices 1`,
		`trmnl_battery_voltage_volts{device_id="ABC123"} 3.95`,
		`trmnl_battery_charging{device_id="ABC123"} 1`,
		`trmnl_wifi_rssi_dbm{device_id="ABC123"} -62`,
		`trmnl_refresh_rate_seconds{device_id="ABC123"} 900`,
		`trmnl_last_seen_timestamp_seconds{device_id="ABC123"}`,
		`model="og_plus"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
}

// A device that has never polled must not report zeroed telemetry.
func TestScrapeSkipsMissingTelemetry(t *testing.T) {
	st := openTest(t)
	if _, err := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:02", APIKey: "k", FriendlyID: "NEW001"}); err != nil {
		t.Fatalf("create device: %v", err)
	}

	body := scrape(t, metrics.New(st, "dev", metrics.Auth{}), "", "").Body.String()
	for _, unwanted := range []string{"trmnl_battery_voltage_volts", "trmnl_wifi_rssi_dbm", "trmnl_last_seen_timestamp_seconds"} {
		if strings.Contains(body, unwanted+"{") {
			t.Errorf("unexpected %s for a never-seen device:\n%s", unwanted, body)
		}
	}
	if !strings.Contains(body, `trmnl_refresh_rate_seconds{device_id="NEW001"} 900`) {
		t.Errorf("refresh rate missing:\n%s", body)
	}
}

func TestBasicAuth(t *testing.T) {
	h := metrics.New(openTest(t), "dev", metrics.Auth{User: "scraper", Password: "s3cret"})

	if got := scrape(t, h, "", "").Code; got != http.StatusUnauthorized {
		t.Errorf("no credentials: got %d, want 401", got)
	}
	if got := scrape(t, h, "scraper", "wrong").Code; got != http.StatusUnauthorized {
		t.Errorf("wrong password: got %d, want 401", got)
	}
	if got := scrape(t, h, "scraper", "s3cret").Code; got != http.StatusOK {
		t.Errorf("valid credentials: got %d, want 200", got)
	}
}
