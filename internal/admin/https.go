package admin

import (
	"net"
	"net/http"
	"strings"

	"github.com/gesellix/go-trmnl/internal/tlscert"
)

// WithHTTPS tells the admin UI about the optional HTTPS listener so it can
// link to it and offer the local CA certificate for download. addr is the
// HTTPS listen address; src may be nil when HTTPS is disabled.
func (h *Handler) WithHTTPS(addr string, src *tlscert.Source) *Handler {
	if addr != "" && src != nil {
		h.httpsAddr, h.tls = addr, src
	}
	return h
}

// httpsURL returns the HTTPS origin for the host the admin is browsing (e.g.
// https://trmnl.fritz.box:8443), or "" when HTTPS is disabled.
func (h *Handler) httpsURL(r *http.Request) string {
	if h.tls == nil {
		return ""
	}
	host := r.Host
	if hn, _, err := net.SplitHostPort(host); err == nil {
		host = hn
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]" // IPv6 literal
	}
	_, port, _ := net.SplitHostPort(h.httpsAddr)
	if port != "" && port != "443" {
		host += ":" + port
	}
	return "https://" + host
}

// TLSCACert serves the local CA certificate so browsers and operating systems
// can be told to trust the HTTPS listener.
func (h *Handler) TLSCACert(w http.ResponseWriter, r *http.Request) {
	if h.tls == nil || !h.tls.LocalCA() {
		http.NotFound(w, r)
		return
	}
	pem, err := h.tls.CACertPEM()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="go-trmnl-ca.crt"`)
	_, _ = w.Write(pem)
}

// httpsInfo collects what the settings page shows about HTTPS.
func (h *Handler) httpsInfo(r *http.Request) map[string]any {
	if h.tls == nil {
		return nil
	}
	info := map[string]any{
		"URL":     h.httpsURL(r),
		"Addr":    h.httpsAddr,
		"LocalCA": h.tls.LocalCA(),
		"Active":  requestIsHTTPS(r),
	}
	if h.tls.LocalCA() {
		info["Hosts"] = strings.Join(h.tls.Hosts(), ", ")
		if pem, err := h.tls.CACertPEM(); err == nil {
			info["Fingerprint"], _ = tlscert.Fingerprint(pem)
		}
	}
	return info
}
