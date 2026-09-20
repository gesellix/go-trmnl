package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any inherited env so defaults are deterministic.
	unsetEnv(t, "TRMNL_LISTEN", "TRMNL_BASE_URL", "TRMNL_DATA_DIR", "TRMNL_DB",
		"TRMNL_UPLOADS", "TRMNL_ADMIN_USER", "TRMNL_ADMIN_PASSWORD")

	c, err := config.Load([]string{"-base-url", "http://192.168.1.10:8080", "-data-dir", "/tmp/x"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != ":8080" {
		t.Errorf("listen = %q", c.ListenAddr)
	}
	if c.DBPath != filepath.Join("/tmp/x", "trmnl.db") {
		t.Errorf("db path derived wrong: %q", c.DBPath)
	}
	if c.UploadsDir != filepath.Join("/tmp/x", "uploads") {
		t.Errorf("uploads derived wrong: %q", c.UploadsDir)
	}
	if c.AdminUser != "admin" || c.AdminPassword != "" {
		t.Errorf("admin defaults wrong: %q/%q", c.AdminUser, c.AdminPassword)
	}
	if c.CleanupInterval != time.Hour {
		t.Errorf("cleanup interval default = %v, want 1h", c.CleanupInterval)
	}
	if c.LogRetention != 32*24*time.Hour {
		t.Errorf("log retention default = %v, want 32d", c.LogRetention)
	}
}

func TestLogRetentionParsing(t *testing.T) {
	cases := map[string]time.Duration{
		"30d":  30 * 24 * time.Hour,
		"720h": 720 * time.Hour,
		"0":    0,
	}
	for in, want := range cases {
		c, err := config.Load([]string{"-base-url", "http://h:8080", "-log-retention", in})
		if err != nil {
			t.Fatalf("Load(%q): %v", in, err)
		}
		if c.LogRetention != want {
			t.Errorf("log-retention %q = %v, want %v", in, c.LogRetention, want)
		}
	}
	if _, err := config.Load([]string{"-base-url", "http://h:8080", "-log-retention", "nope"}); err == nil {
		t.Error("expected error for invalid log-retention")
	}
}

func TestCleanupInterval(t *testing.T) {
	c, err := config.Load([]string{"-base-url", "http://h:8080", "-cleanup-interval", "0"})
	if err != nil {
		t.Fatal(err)
	}
	if c.CleanupInterval != 0 {
		t.Errorf("cleanup interval = %v, want 0 (disabled)", c.CleanupInterval)
	}
	if _, err := config.Load([]string{"-base-url", "http://h:8080", "-cleanup-interval", "nope"}); err == nil {
		t.Error("expected error for invalid cleanup-interval")
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	t.Setenv("TRMNL_LISTEN", ":9000")
	t.Setenv("TRMNL_BASE_URL", "http://env:9000")

	c, err := config.Load([]string{"-listen", ":7000"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != ":7000" {
		t.Errorf("flag should win: listen = %q", c.ListenAddr)
	}
	// base-url not passed as a flag, so the env value applies.
	if c.PublicBaseURL != "http://env:9000" {
		t.Errorf("env base-url not applied: %q", c.PublicBaseURL)
	}
}

func TestBaseURLTrailingSlashTrimmed(t *testing.T) {
	c, err := config.Load([]string{"-base-url", "http://host:8080/"})
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicBaseURL != "http://host:8080" {
		t.Errorf("trailing slash not trimmed: %q", c.PublicBaseURL)
	}
}

func TestRelativeBaseURLRejected(t *testing.T) {
	if _, err := config.Load([]string{"-base-url", "/relative"}); err == nil {
		t.Error("expected error for non-absolute base URL")
	}
}

func TestLoopbackWarning(t *testing.T) {
	loop, _ := config.Load([]string{"-base-url", "http://127.0.0.1:8080"})
	if loop.LoopbackWarning() == "" {
		t.Error("expected loopback warning for 127.0.0.1")
	}
	lan, _ := config.Load([]string{"-base-url", "http://192.168.1.50:8080"})
	if lan.LoopbackWarning() != "" {
		t.Errorf("unexpected warning for LAN IP: %q", lan.LoopbackWarning())
	}
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	c, err := config.Load([]string{"-base-url", "http://host:8080", "-data-dir", filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	for _, d := range []string{c.DataDir, c.UploadsDir} {
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			t.Errorf("dir %q not created (err=%v)", d, err)
		}
	}
}

// unsetEnv removes the named environment variables for the duration of the
// test, restoring any prior values on cleanup.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			t.Cleanup(func() { _ = os.Setenv(k, v) })
		}
		_ = os.Unsetenv(k)
	}
}

func TestHTTPSSettings(t *testing.T) {
	unsetEnv(t, "TRMNL_HTTPS_LISTEN", "TRMNL_TLS_HOSTS", "TRMNL_TLS_CERT", "TRMNL_TLS_KEY")
	base := []string{"-base-url", "http://192.168.1.10:8080", "-data-dir", "/tmp/x"}

	c, err := config.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPSListenAddr != "" || c.TLSHosts != nil {
		t.Errorf("HTTPS should be off by default: %+v", c)
	}

	t.Setenv("TRMNL_HTTPS_LISTEN", ":9443")
	t.Setenv("TRMNL_TLS_HOSTS", " trmnl.fritz.box, ,pi.home.arpa ")
	c, err = config.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPSListenAddr != ":9443" || len(c.TLSHosts) != 2 || c.TLSHosts[1] != "pi.home.arpa" {
		t.Errorf("env not applied: %q %q", c.HTTPSListenAddr, c.TLSHosts)
	}
	if c.TLSDir() != filepath.Join("/tmp/x", "tls") {
		t.Errorf("TLSDir = %q", c.TLSDir())
	}

	for name, args := range map[string][]string{
		"cert without key":       {"-tls-cert", "c.pem"},
		"invalid listen address": {"-https-listen", "9443"},
		"hosts without listener": {"-https-listen", "", "-tls-hosts", "a.example.com"},
		"cert without listener":  {"-https-listen", "", "-tls-hosts", "", "-tls-cert", "c.pem", "-tls-key", "k.pem"},
	} {
		if _, err := config.Load(append(append([]string{}, base...), args...)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestMetricsSettings(t *testing.T) {
	unsetEnv(t, "TRMNL_NO_METRICS", "TRMNL_METRICS_LISTEN", "TRMNL_METRICS_USER", "TRMNL_METRICS_PASSWORD")
	base := []string{"-base-url", "http://192.168.1.10:8080", "-data-dir", "/tmp/x"}

	c, err := config.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	if c.DisableMetrics || c.MetricsListenAddr != "" || c.MetricsUser != "metrics" || c.MetricsPassword != "" {
		t.Errorf("metrics defaults wrong: %+v", c)
	}

	t.Setenv("TRMNL_METRICS_LISTEN", " 127.0.0.1:9090 ")
	t.Setenv("TRMNL_METRICS_PASSWORD", "s3cret")
	c, err = config.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	if c.MetricsListenAddr != "127.0.0.1:9090" || c.MetricsPassword != "s3cret" {
		t.Errorf("env not applied: %q %q", c.MetricsListenAddr, c.MetricsPassword)
	}

	for name, args := range map[string][]string{
		"invalid listen address":   {"-metrics-listen", "9090"},
		"clashes with HTTP listen": {"-metrics-listen", ":8080"},
		"listener with no-metrics": {"-no-metrics"},
	} {
		if _, err := config.Load(append(append([]string{}, base...), args...)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
