// Command trmnld is a self-hosted BYOS (Build Your Own Server) for the TRMNL
// e-ink display device.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	// Embed the timezone database so timezone-aware screens (e.g. the clock
	// plugin) work on minimal images that lack system tzdata.
	_ "time/tzdata"

	"github.com/gesellix/go-trmnl/internal/admin"
	"github.com/gesellix/go-trmnl/internal/calendar"
	"github.com/gesellix/go-trmnl/internal/config"
	"github.com/gesellix/go-trmnl/internal/deviceapi"
	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/plugins/familycalendar"
	"github.com/gesellix/go-trmnl/internal/secret"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"
	"github.com/gesellix/go-trmnl/internal/uploads"

	// Register built-in screen plugins.
	_ "github.com/gesellix/go-trmnl/internal/plugins/clock"
	_ "github.com/gesellix/go-trmnl/internal/plugins/daysleft"
	_ "github.com/gesellix/go-trmnl/internal/plugins/quote"
	_ "github.com/gesellix/go-trmnl/internal/plugins/staticimage"
	_ "github.com/gesellix/go-trmnl/internal/plugins/weather"
)

// version is the build version, overridden at release time via
// -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatalf("trmnld: %v", err)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}
	if err = cfg.EnsureDirs(); err != nil {
		return err
	}
	if w := cfg.LoopbackWarning(); w != "" {
		log.Printf("WARNING: %s", w)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	if seedErr := seedExample(st); seedErr != nil {
		log.Printf("WARNING: seed example content: %v", seedErr)
	}

	// Family calendar: shared account store + provider sync. The OAuth redirect
	// URI must match the one registered in the Google Cloud OAuth client.
	googleOAuth := calendar.NewGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.PublicBaseURL+"/admin/oauth/google/callback")
	secretBox := secret.NewFromEnv()
	cal := calendar.NewService(st, googleOAuth, secretBox)
	plugins.Register(familycalendar.New(cal))
	if !googleOAuth.Configured() {
		log.Printf("note: family calendar Google integration disabled (set TRMNL_GOOGLE_CLIENT_ID/SECRET to enable)")
	} else if !secretBox.Enabled() {
		log.Printf("note: calendar OAuth tokens are stored unencrypted (set TRMNL_SECRET_KEY to enable encryption at rest)")
	}

	r := server.New()

	// Firmware-facing device API.
	deviceapi.New(st, cfg.PublicBaseURL, cfg.UploadsDir).Routes(r)

	// Rendered images, fetched by the device on the LAN.
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))

	// Admin web UI (and "/" redirect to it).
	if cfg.AdminPassword == "" {
		log.Printf("WARNING: admin UI authentication is disabled (set -admin-password or TRMNL_ADMIN_PASSWORD)")
	}
	admin.New(st, cfg.PublicBaseURL, cfg.UploadsDir, admin.Auth{
		User:     cfg.AdminUser,
		Password: cfg.AdminPassword,
	}, cal).Routes(r)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.CleanupInterval > 0 || cfg.LogRetention > 0 {
		go runMaintenance(ctx, st, cfg.UploadsDir, cfg.CleanupInterval, cfg.LogRetention)
	}
	if googleOAuth.Configured() {
		go runCalendarSync(ctx, cal)
	}

	log.Printf("trmnld %s listening on %s (public base URL %s)", version, cfg.ListenAddr, cfg.PublicBaseURL)
	return server.Run(ctx, cfg.ListenAddr, r)
}

// runMaintenance periodically prunes the rendered-image cache (files not
// referenced by any screen, older than one tick as a grace window) and old
// device logs. It runs until ctx is cancelled. The tick is cleanupInterval when
// set, otherwise a default used solely to pace log pruning.
func runMaintenance(ctx context.Context, st *store.Store, uploadsDir string, cleanupInterval, logRetention time.Duration) {
	tick := cleanupInterval
	if tick <= 0 {
		tick = time.Hour
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if cleanupInterval > 0 {
				sweepCache(st, uploadsDir, cleanupInterval)
				sweepAssets(st, filepath.Join(uploadsDir, "assets"), cleanupInterval)
			}
			if logRetention > 0 {
				if n, err := st.PruneLogs(logRetention); err != nil {
					log.Printf("maintenance: prune logs: %v", err)
				} else if n > 0 {
					log.Printf("maintenance: pruned %d old log entry(ies)", n)
				}
			}
		}
	}
}

// runCalendarSync periodically refreshes the calendar event cache. Each account
// is synced no more often than its own refresh interval; the loop wakes on the
// smallest configured interval (clamped) so newly added or manually changed
// accounts are picked up promptly. It runs until ctx is cancelled.
func runCalendarSync(ctx context.Context, cal *calendar.Service) {
	// Initial sync shortly after startup so screens have data without waiting.
	if err := cal.SyncDue(ctx, time.Now()); err != nil {
		log.Printf("calendar sync: %v", err)
	}
	for {
		timer := time.NewTimer(cal.NextWake())
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if err := cal.SyncDue(ctx, time.Now()); err != nil {
				log.Printf("calendar sync: %v", err)
			}
		}
	}
}

// sweepCache removes rendered-image files not referenced by any screen.
func sweepCache(st *store.Store, uploadsDir string, grace time.Duration) {
	hashes, err := st.ActiveRenderHashes()
	if err != nil {
		log.Printf("maintenance: list render hashes: %v", err)
		return
	}
	keep := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		keep[h] = true
	}
	removed, err := uploads.Sweep(uploadsDir, keep, grace)
	if err != nil {
		log.Printf("maintenance: sweep %s: %v", uploadsDir, err)
		return
	}
	if removed > 0 {
		log.Printf("maintenance: removed %d stale cache file(s)", removed)
	}
}

// sweepAssets removes uploaded image assets no longer referenced by any
// static-image screen (e.g. orphaned by a re-upload or a deleted screen).
func sweepAssets(st *store.Store, assetsDir string, grace time.Duration) {
	settings, err := st.ScreenSettings("staticimage")
	if err != nil {
		log.Printf("maintenance: list staticimage settings: %v", err)
		return
	}
	keep := make(map[string]bool, len(settings))
	for _, s := range settings {
		var v struct {
			File string `json:"file"`
		}
		if json.Unmarshal([]byte(s), &v) == nil && v.File != "" {
			keep[filepath.Base(v.File)] = true
		}
	}
	removed, err := uploads.SweepAssets(assetsDir, keep, grace)
	if err != nil {
		log.Printf("maintenance: sweep assets: %v", err)
		return
	}
	if removed > 0 {
		log.Printf("maintenance: removed %d unused asset(s)", removed)
	}
}
