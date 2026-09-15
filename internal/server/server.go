// Package server assembles the chi router and runs the HTTP server with
// graceful shutdown.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// New builds the root chi router with common middleware and a health check.
// Feature routes (device API, uploads, admin) are mounted by the caller via
// the returned router before it is served.
func New() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return r
}

// Listener describes one address the server binds to. A non-nil TLS config
// serves HTTPS on it.
type Listener struct {
	Addr string
	TLS  *tls.Config
}

// Run serves handler on every listener until ctx is cancelled, then shuts all
// of them down gracefully. If any listener fails, the others are shut down and
// the error is returned.
func Run(ctx context.Context, handler http.Handler, listeners ...Listener) error {
	servers := make([]*http.Server, 0, len(listeners))
	errCh := make(chan error, len(listeners))
	for _, l := range listeners {
		srv := &http.Server{
			Addr:              l.Addr,
			Handler:           handler,
			TLSConfig:         l.TLS,
			ReadHeaderTimeout: 10 * time.Second,
			// WriteTimeout bounds slow/stuck responses (image downloads are small
			// LAN transfers); IdleTimeout reaps idle keep-alive connections so they
			// don't accumulate over a long-running process.
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		servers = append(servers, srv)
		go func(srv *http.Server) {
			var err error
			if srv.TLSConfig != nil {
				// Certificates come from TLSConfig.GetCertificate.
				err = srv.ListenAndServeTLS("", "")
			} else {
				err = srv.ListenAndServe()
			}
			if err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("listen %s: %w", srv.Addr, err)
			}
		}(srv)
	}

	var runErr error
	select {
	case runErr = <-errCh:
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil && runErr == nil {
			runErr = err
		}
	}
	return runErr
}
