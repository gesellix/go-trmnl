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
