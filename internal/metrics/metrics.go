// Package metrics exposes a Prometheus /metrics endpoint. Device gauges are
// collected from the store on scrape (not cached in memory), so the values
// survive a restart and reflect whatever the last display poll persisted.
package metrics

import (
	"crypto/subtle"
	"log"
	"net/http"

	"github.com/gesellix/go-trmnl/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Auth holds the HTTP Basic Auth credentials for /metrics. They are deliberately
// separate from the admin credentials: a scraper should not hold admin access.
// When Password is empty, authentication is disabled.
type Auth struct {
	User     string
	Password string
}

// Handler serves the metrics endpoint.
type Handler struct {
	registry *prometheus.Registry
	auth     Auth
}

// New builds a metrics handler with its own registry: the device collector,
// plus the standard Go runtime and process collectors. version is reported as
// a label on trmnl_build_info.
func New(st *store.Store, version string, auth Auth) *Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		newDeviceCollector(st),
		buildInfo(version),
	)
	return &Handler{registry: reg, auth: auth}
}

// Routes mounts GET /metrics onto r.
func (h *Handler) Routes(r chi.Router) {
	r.With(h.requireAuth).Method(http.MethodGet, "/metrics", promhttp.HandlerFor(h.registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	}))
}

// requireAuth enforces HTTP Basic Auth when a password is configured.
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.auth.Password == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(user), []byte(h.auth.User)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(h.auth.Password)) == 1
		if !ok || !userOK || !passOK {
			w.Header().Set("WWW-Authenticate", `Basic realm="go-trmnl metrics", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func buildInfo(version string) prometheus.Collector {
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "trmnl_build_info",
		Help: "Build information of the running server; always 1.",
	}, []string{"version"})
	g.WithLabelValues(version).Set(1)
	return g
}

// deviceLabels are the labels of every per-device metric. device_id is the
// device's friendly ID, which is stable and shown in the admin UI.
var deviceLabels = []string{"device_id"}

func deviceDesc(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(name, help, append(append([]string{}, deviceLabels...), labels...), nil)
}

// deviceCollector reports the last telemetry persisted per device. It queries
// the store on every scrape, so there is no state to keep in sync with the
// device API handlers.
type deviceCollector struct {
	store *store.Store

	up             *prometheus.Desc
	info           *prometheus.Desc
	lastSeen       *prometheus.Desc
	batteryVoltage *prometheus.Desc
	batteryCharge  *prometheus.Desc
	rssi           *prometheus.Desc
	refreshRate    *prometheus.Desc
	scrapeErrors   prometheus.Counter
}

func newDeviceCollector(st *store.Store) *deviceCollector {
	return &deviceCollector{
		store: st,
		up: prometheus.NewDesc("trmnl_devices",
			"Number of registered devices.", nil, nil),
		info: deviceDesc("trmnl_device_info",
			"Static device information; always 1.", "name", "model", "firmware_version"),
		lastSeen: deviceDesc("trmnl_last_seen_timestamp_seconds",
			"Unix timestamp of the device's last display poll."),
		batteryVoltage: deviceDesc("trmnl_battery_voltage_volts",
			"Battery voltage reported on the last display poll."),
		batteryCharge: deviceDesc("trmnl_battery_charging",
			"Whether the device reported itself as charging on the last display poll (1) or not (0)."),
		rssi: deviceDesc("trmnl_wifi_rssi_dbm",
			"WiFi signal strength reported on the last display poll."),
		refreshRate: deviceDesc("trmnl_refresh_rate_seconds",
			"Refresh interval the server hands out to the device, as configured in the admin UI."),
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "trmnl_metrics_scrape_errors_total",
			Help: "Number of scrapes that failed to read devices from the store.",
		}),
	}
}

func (c *deviceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.info
	ch <- c.lastSeen
	ch <- c.batteryVoltage
	ch <- c.batteryCharge
	ch <- c.rssi
	ch <- c.refreshRate
	c.scrapeErrors.Describe(ch)
}

func (c *deviceCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() { c.scrapeErrors.Collect(ch) }()

	devices, err := c.store.ListDevices()
	if err != nil {
		c.scrapeErrors.Inc()
		log.Printf("metrics: list devices: %v", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, float64(len(devices)))

	for _, d := range devices {
		id := d.FriendlyID
		ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, 1,
			id, d.Name.String, d.Model.String, d.FWVersion.String)
		ch <- prometheus.MustNewConstMetric(c.refreshRate, prometheus.GaugeValue, float64(d.RefreshRate), id)
		// Telemetry fields are absent until the device has polled at least once;
		// skipping them keeps a never-seen device from looking like 0 V.
		if d.LastSeenAt.Valid {
			ch <- prometheus.MustNewConstMetric(c.lastSeen, prometheus.GaugeValue, float64(d.LastSeenAt.Int64), id)
		}
		if d.BatteryVoltage.Valid {
			ch <- prometheus.MustNewConstMetric(c.batteryVoltage, prometheus.GaugeValue, d.BatteryVoltage.Float64, id)
		}
		if d.BatteryCharging.Valid {
			ch <- prometheus.MustNewConstMetric(c.batteryCharge, prometheus.GaugeValue, boolValue(d.BatteryCharging.Bool), id)
		}
		if d.RSSI.Valid {
			ch <- prometheus.MustNewConstMetric(c.rssi, prometheus.GaugeValue, float64(d.RSSI.Int64), id)
		}
	}
}

func boolValue(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// Mux returns a standalone handler serving only /metrics, for the dedicated
// metrics listener. Scrapes are not request-logged there, unlike on the shared
// listener.
func (h *Handler) Mux() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	h.Routes(r)
	return r
}
