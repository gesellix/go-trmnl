// Command trmnld is a self-hosted BYOS (Build Your Own Server) for the TRMNL
// e-ink display device.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gesellix/go-trmnl/internal/config"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"
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
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}
	if w := cfg.LoopbackWarning(); w != "" {
		log.Printf("WARNING: %s", w)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	r := server.New()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("trmnld listening on %s (public base URL %s)", cfg.ListenAddr, cfg.PublicBaseURL)
	return server.Run(ctx, cfg.ListenAddr, r)
}
