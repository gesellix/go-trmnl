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

	// Embed the timezone database so timezone-aware screens (e.g. the clock
	// plugin) work on minimal images that lack system tzdata.
	_ "time/tzdata"

	"github.com/gesellix/go-trmnl/internal/admin"
	"github.com/gesellix/go-trmnl/internal/config"
	"github.com/gesellix/go-trmnl/internal/deviceapi"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"

	// Register built-in screen plugins.
	_ "github.com/gesellix/go-trmnl/internal/plugins/clock"
	_ "github.com/gesellix/go-trmnl/internal/plugins/staticimage"
	_ "github.com/gesellix/go-trmnl/internal/plugins/weather"
)

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

	log.Printf("trmnld listening on %s (public base URL %s)", cfg.ListenAddr, cfg.PublicBaseURL)
	return server.Run(ctx, cfg.ListenAddr, r)
}
