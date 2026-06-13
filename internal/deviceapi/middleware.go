package deviceapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"

	"github.com/gesellix/go-trmnl/internal/device"
	"github.com/gesellix/go-trmnl/internal/httpx"
	"github.com/gesellix/go-trmnl/internal/store"
)

type ctxKey int

const (
	ctxKeyMAC ctxKey = iota
	ctxKeyDevice
)

// macFromCtx returns the normalized MAC stored by parseMAC/loadDevice.
func macFromCtx(ctx context.Context) string {
	mac, _ := ctx.Value(ctxKeyMAC).(string)
	return mac
}

// deviceFromCtx returns the device stored by loadDevice.
func deviceFromCtx(ctx context.Context) *store.Device {
	d, _ := ctx.Value(ctxKeyDevice).(*store.Device)
	return d
}

// parseMAC validates the ID header and stores the normalized MAC in context.
// It does not require the device to exist (used by /api/setup).
func (h *Handler) parseMAC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("ID")
		if raw == "" {
			log.Printf("deviceapi: missing ID header in request to %s from %s", r.URL.Path, r.RemoteAddr)
			httpx.WriteProblem(w, http.StatusUnprocessableEntity, "/problem#device_id",
				"unprocessable_content", "Missing ID header.", r.URL.Path,
				map[string]any{"errors": map[string]any{"ID": []string{"is missing"}}})
			return
		}
		mac, err := device.NormalizeMAC(raw)
		if err != nil {
			log.Printf("deviceapi: invalid MAC address %q in ID header from %s", raw, r.RemoteAddr)
			httpx.WriteProblem(w, http.StatusUnprocessableEntity, "/problem#device_id",
				"unprocessable_content", "Invalid MAC address in ID header.", r.URL.Path, nil)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyMAC, mac)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loadDevice validates the ID header and loads the device.
func (h *Handler) loadDevice(next http.Handler) http.Handler {
	return h.parseMAC(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mac := macFromCtx(r.Context())
		d, err := h.store.GetDeviceByMAC(mac)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				log.Printf("deviceapi: unknown device MAC %s from %s calling %s", mac, r.RemoteAddr, r.URL.Path)
				httpx.WriteProblem(w, http.StatusNotFound, "/problem#device_id",
					"not_found", "Invalid device ID.", r.URL.Path, nil)
				return
			}
			httpx.WriteProblem(w, http.StatusInternalServerError, "/problem#internal",
				"internal_server_error", "Failed to load device.", r.URL.Path, nil)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyDevice, d)
		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}

// requireToken checks the Access-Token header against the device's api_key
// using a constant-time comparison. Must run after loadDevice.
// If the token is invalid, it returns a 401 (unauthorized) which suggests the
// device should probably try /api/setup again.
func (h *Handler) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.disableAuth {
			next.ServeHTTP(w, r)
			return
		}
		d := deviceFromCtx(r.Context())
		token := r.Header.Get("Access-Token")
		if d == nil {
			httpx.WriteProblem(w, http.StatusUnauthorized, "/problem#device_id",
				"unauthorized", "Invalid or missing Access-Token.", r.URL.Path, nil)
			return
		}

		valid := subtle.ConstantTimeCompare([]byte(token), []byte(d.APIKey)) == 1
		if !valid {
			log.Printf("deviceapi: invalid access token for device %s from %s", d.MAC, r.RemoteAddr)
			httpx.WriteProblem(w, http.StatusUnauthorized, "/problem#device_id",
				"unauthorized", "Invalid or missing Access-Token.", r.URL.Path, nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}
