// Command trmnld is a self-hosted BYOS (Build Your Own Server) for the TRMNL
// e-ink display device.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Embed the timezone database so timezone-aware screens (e.g. the clock
	// plugin) work on minimal images that lack system tzdata.
	_ "time/tzdata"

	"github.com/gesellix/go-trmnl/internal/admin"
	"github.com/gesellix/go-trmnl/internal/config"
	"github.com/gesellix/go-trmnl/internal/deviceapi"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"
	"github.com/gesellix/go-trmnl/internal/uploads"

	// Register built-in screen plugins.
	_ "github.com/gesellix/go-trmnl/internal/plugins/clock"
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
	}).Routes(r)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.CleanupInterval > 0 {
		go runCleanup(ctx, st, cfg.UploadsDir, cfg.CleanupInterval)
	}

	log.Printf("trmnld %s listening on %s (public base URL %s)", version, cfg.ListenAddr, cfg.PublicBaseURL)
	return server.Run(ctx, cfg.ListenAddr, r)
}

// runCleanup periodically prunes the rendered-image cache of files not
// referenced by any screen. Files newer than one interval are kept as a grace
// window. Runs until ctx is cancelled.
func runCleanup(ctx context.Context, st *store.Store, uploadsDir string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hashes, err := st.ActiveRenderHashes()
			if err != nil {
				log.Printf("cleanup: list render hashes: %v", err)
				continue
			}
			keep := make(map[string]bool, len(hashes))
			for _, h := range hashes {
				keep[h] = true
			}
			removed, err := uploads.Sweep(uploadsDir, keep, interval)
			if err != nil {
				log.Printf("cleanup: sweep %s: %v", uploadsDir, err)
				continue
			}
			if removed > 0 {
				log.Printf("cleanup: removed %d stale cache file(s)", removed)
			}
		}
	}
}
