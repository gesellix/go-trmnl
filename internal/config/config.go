// Package config loads server configuration from command-line flags with
// environment-variable fallbacks. It intentionally depends on no other
// internal package.
package config

import (
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the server.
type Config struct {
	// ListenAddr is the TCP address the HTTP server binds to, e.g. ":8080".
	ListenAddr string
	// PublicBaseURL is the absolute URL the device uses to reach this server.
	// It must be LAN-reachable by the device (not a loopback address), because
	// the device fetches rendered images from <PublicBaseURL>/uploads/...
	PublicBaseURL string
	// DataDir is the root directory for persistent state.
	DataDir string
	// DBPath is the SQLite database file path.
	DBPath string
	// UploadsDir is where rendered images are cached and served from.
	UploadsDir string
	// AdminUser and AdminPassword guard the /admin UI with HTTP Basic Auth.
	// Auth is disabled when AdminPassword is empty.
	AdminUser     string
	AdminPassword string
	// CleanupInterval is how often the rendered-image cache is pruned of
	// unreferenced files. Zero disables cleanup.
	CleanupInterval time.Duration
	// LogRetention is how long device log entries are kept before pruning.
	// Zero disables log pruning.
	LogRetention time.Duration
	// SecretKey encrypts sensitive calendar credentials at rest (OAuth client
	// secrets and tokens, CalDAV passwords). Empty stores them as plaintext.
	SecretKey string
}

// Load parses configuration from the given args (typically os.Args[1:]),
// applying environment-variable defaults. Flags take precedence over env vars.
func Load(args []string) (*Config, error) {
	fs := flag.NewFlagSet("trmnld", flag.ContinueOnError)

	listen := fs.String("listen", env("TRMNL_LISTEN", ":8080"), "HTTP listen address")
	baseURL := fs.String("base-url", env("TRMNL_BASE_URL", ""), "Public base URL reachable by the device (e.g. http://192.168.1.10:8080)")
	dataDir := fs.String("data-dir", env("TRMNL_DATA_DIR", "./data"), "Data directory for database and uploads")
	dbPath := fs.String("db", env("TRMNL_DB", ""), "SQLite database path (default <data-dir>/trmnl.db)")
	uploads := fs.String("uploads", env("TRMNL_UPLOADS", ""), "Uploads directory (default <data-dir>/uploads)")
	adminUser := fs.String("admin-user", env("TRMNL_ADMIN_USER", "admin"), "Admin UI username")
	adminPass := fs.String("admin-password", env("TRMNL_ADMIN_PASSWORD", ""), "Admin UI password (empty disables auth)")
	cleanup := fs.String("cleanup-interval", env("TRMNL_CLEANUP_INTERVAL", "1h"), "How often to prune the rendered-image cache (e.g. 1h); 0 disables")
	logRetention := fs.String("log-retention", env("TRMNL_LOG_RETENTION", "32d"), "How long to keep device logs (e.g. 32d, 720h); 0 disables")
	secretKey := fs.String("secret-key", env("TRMNL_SECRET_KEY", ""), "Key to encrypt calendar credentials at rest (empty stores them as plaintext)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cleanupInterval, perr := parseDuration(*cleanup)
	if perr != nil {
		return nil, fmt.Errorf("invalid cleanup-interval %q: %w", *cleanup, perr)
	}
	logRetentionDur, perr := parseDuration(*logRetention)
	if perr != nil {
		return nil, fmt.Errorf("invalid log-retention %q: %w", *logRetention, perr)
	}

	c := &Config{
		ListenAddr:      *listen,
		PublicBaseURL:   strings.TrimRight(*baseURL, "/"),
		DataDir:         *dataDir,
		DBPath:          *dbPath,
		UploadsDir:      *uploads,
		AdminUser:       *adminUser,
		AdminPassword:   *adminPass,
		CleanupInterval: cleanupInterval,
		LogRetention:    logRetentionDur,
		SecretKey:       *secretKey,
	}

	if c.PublicBaseURL == "" {
		c.PublicBaseURL = guessBaseURL(c.ListenAddr)
	}
	if c.DBPath == "" {
		c.DBPath = filepath.Join(c.DataDir, "trmnl.db")
	}
	if c.UploadsDir == "" {
		c.UploadsDir = filepath.Join(c.DataDir, "uploads")
	}

	if u, err := url.Parse(c.PublicBaseURL); err != nil || !u.IsAbs() {
		return nil, fmt.Errorf("base-url must be an absolute URL, got %q", c.PublicBaseURL)
	}
	return c, nil
}

// EnsureDirs creates the data and uploads directories if they do not exist.
func (c *Config) EnsureDirs() error {
	for _, d := range []string{c.DataDir, c.UploadsDir, filepath.Dir(c.DBPath)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return nil
}

// LoopbackWarning returns a non-empty message if the configured base URL points
// at a loopback address, which a physical device on the LAN cannot reach.
func (c *Config) LoopbackWarning() string {
	u, err := url.Parse(c.PublicBaseURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host == "localhost" {
		return loopbackMsg(c.PublicBaseURL)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return loopbackMsg(c.PublicBaseURL)
	}
	return ""
}

func loopbackMsg(u string) string {
	return fmt.Sprintf("base-url %q is a loopback address; a physical TRMNL device on the LAN will not be able to fetch images. Set -base-url to this host's LAN IP.", u)
}

func guessBaseURL(listenAddr string) string {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil || port == "" {
		port = "8080"
	}
	if ip := outboundIP(); ip != "" {
		return fmt.Sprintf("http://%s:%s", ip, port)
	}
	return fmt.Sprintf("http://localhost:%s", port)
}

// outboundIP returns the preferred outbound IPv4 address of this host, or "".
// It uses a UDP dial which does not actually send packets.
func outboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

// parseDuration parses a Go duration, additionally accepting a plain
// integer-days form like "32d" (which time.ParseDuration does not support).
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid days duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
