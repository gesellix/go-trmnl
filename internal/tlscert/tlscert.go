// Package tlscert provides the certificate for trmnld's optional HTTPS
// listener: either a user-supplied certificate/key pair, or a server
// certificate issued by a local CA that is generated and persisted on first use.
//
// The local CA exists so that browsers can reach the admin UI over HTTPS on a
// LAN hostname (Google OAuth requires HTTPS redirect URIs for apps "In
// production"). Browsers trust it once its certificate is installed; the
// certificate is downloadable from the admin UI.
package tlscert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	caValidity = 10 * 365 * 24 * time.Hour
	// serverValidity stays below the 398-day limit some clients (notably Apple
	// platforms) enforce for TLS server certificates.
	serverValidity = 397 * 24 * time.Hour
	// renewBefore re-issues the server certificate this long before it expires.
	renewBefore = 30 * 24 * time.Hour
	// recheckEvery bounds how often a running server re-validates (and, for
	// user-supplied files, reloads) its certificate.
	recheckEvery = time.Hour

	caCertFile     = "ca.crt"
	caKeyFile      = "ca.key"
	serverCertFile = "server.crt"
	serverKeyFile  = "server.key"
)

// Source provides the HTTPS server certificate.
type Source struct {
	dir      string   // local CA mode: directory holding the CA and server cert
	hosts    []string // local CA mode: DNS names and IPs the server cert must cover
	certFile string   // file mode: user-supplied certificate
	keyFile  string   // file mode: user-supplied key
	now      func() time.Time

	mu        sync.Mutex
	cert      *tls.Certificate
	checkedAt time.Time
	fileMod   time.Time // file mode: newest mtime of certFile/keyFile when loaded
}

// NewLocalCA returns a Source issuing a server certificate for hosts from a
// local CA stored in dir. Both are created on first use.
func NewLocalCA(dir string, hosts []string) *Source {
	return &Source{dir: dir, hosts: normalizeHosts(hosts), now: time.Now}
}

// NewFromFiles returns a Source serving a user-supplied certificate and key.
// The files are reloaded when they change, so renewed certificates are picked
// up without a restart.
func NewFromFiles(certFile, keyFile string) *Source {
	return &Source{certFile: certFile, keyFile: keyFile, now: time.Now}
}

// LocalCA reports whether the certificate comes from the built-in local CA.
func (s *Source) LocalCA() bool { return s.certFile == "" }

// Hosts returns the names and addresses the local CA certificate covers.
func (s *Source) Hosts() []string { return append([]string(nil), s.hosts...) }

// TLSConfig returns a server TLS config backed by this source. It loads (or
// creates) the certificate once up front so configuration errors surface at
// startup rather than on the first handshake.
func (s *Source) TLSConfig() (*tls.Config, error) {
	if _, err := s.certificate(); err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return s.certificate()
		},
	}, nil
}

// CACertPEM returns the local CA certificate in PEM form, creating the CA if
// needed. It fails in file mode, where there is no local CA.
func (s *Source) CACertPEM() ([]byte, error) {
	if !s.LocalCA() {
		return nil, errors.New("tlscert: no local CA (a certificate file is configured)")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ca, _, err := s.ensureCA()
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw}), nil
}

// Fingerprint formats the SHA-256 fingerprint of a PEM certificate as
// colon-separated hex, the form browsers and OS certificate dialogs show.
func Fingerprint(certPEM []byte) (string, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", errors.New("tlscert: no PEM certificate")
	}
	sum := sha256.Sum256(block.Bytes)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":"), nil
}

// certificate returns the current certificate, re-validating it at most once
// per recheckEvery.
func (s *Source) certificate() (*tls.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.cert != nil && now.Sub(s.checkedAt) < recheckEvery {
		return s.cert, nil
	}
	var err error
	if s.LocalCA() {
		err = s.loadOrIssue(now)
	} else {
		err = s.loadFiles()
	}
	if err != nil {
		if s.cert != nil {
			// Keep serving the previous certificate rather than failing handshakes.
			return s.cert, nil
		}
		return nil, err
	}
	s.checkedAt = now
	return s.cert, nil
}

func (s *Source) loadFiles() error {
	mod, err := newestModTime(s.certFile, s.keyFile)
	if err != nil {
		return fmt.Errorf("tlscert: %w", err)
	}
	if s.cert != nil && !mod.After(s.fileMod) {
		return nil
	}
	cert, err := tls.LoadX509KeyPair(s.certFile, s.keyFile)
	if err != nil {
		return fmt.Errorf("tlscert: load %s / %s: %w", s.certFile, s.keyFile, err)
	}
	s.cert, s.fileMod = &cert, mod
	return nil
}

// loadOrIssue loads the stored server certificate, issuing a new one when it
// is missing, not from the current CA, missing a host, or close to expiry.
func (s *Source) loadOrIssue(now time.Time) error {
	ca, caKey, err := s.ensureCA()
	if err != nil {
		return err
	}
	certPath := filepath.Join(s.dir, serverCertFile)
	keyPath := filepath.Join(s.dir, serverKeyFile)
	if cert, lerr := tls.LoadX509KeyPair(certPath, keyPath); lerr == nil && s.usable(cert, ca, now) {
		s.cert = &cert
		return nil
	}

	certPEM, keyPEM, err := issueServer(ca, caKey, s.hosts, now)
	if err != nil {
		return err
	}
	if err = writeFile(keyPath, keyPEM, 0o600); err != nil {
		return err
	}
	if err = writeFile(certPath, certPEM, 0o644); err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("tlscert: %w", err)
	}
	s.cert = &cert
	return nil
}

func (s *Source) usable(cert tls.Certificate, ca *x509.Certificate, now time.Time) bool {
	if len(cert.Certificate) == 0 {
		return false
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil || leaf.CheckSignatureFrom(ca) != nil {
		return false
	}
	if now.Before(leaf.NotBefore) || now.Add(renewBefore).After(leaf.NotAfter) {
		return false
	}
	for _, h := range s.hosts {
		if leaf.VerifyHostname(h) != nil {
			return false
		}
	}
	return true
}

// ensureCA loads the local CA, generating it when absent. Callers hold s.mu.
func (s *Source) ensureCA() (*x509.Certificate, crypto.Signer, error) {
	certPath := filepath.Join(s.dir, caCertFile)
	keyPath := filepath.Join(s.dir, caKeyFile)

	certPEM, certErr := os.ReadFile(certPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	switch {
	case certErr == nil && keyErr == nil:
		return parseCA(certPEM, keyPEM)
	case errors.Is(certErr, os.ErrNotExist) && errors.Is(keyErr, os.ErrNotExist):
		// generate below
	case certErr != nil && !errors.Is(certErr, os.ErrNotExist):
		return nil, nil, fmt.Errorf("tlscert: read %s: %w", certPath, certErr)
	case keyErr != nil && !errors.Is(keyErr, os.ErrNotExist):
		return nil, nil, fmt.Errorf("tlscert: read %s: %w", keyPath, keyErr)
	default:
		// Refuse to silently replace half a CA: browsers may already trust it.
		return nil, nil, fmt.Errorf("tlscert: only one of %s and %s exists; restore or remove both", certPath, keyPath)
	}

	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("tlscert: %w", err)
	}
	certPEM, keyPEM, err := generateCA(s.now())
	if err != nil {
		return nil, nil, err
	}
	if err = writeFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, nil, err
	}
	if err = writeFile(certPath, certPEM, 0o644); err != nil {
		return nil, nil, err
	}
	return parseCA(certPEM, keyPEM)
}

func generateCA(now time.Time) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	name := "go-trmnl local CA"
	if host, herr := os.Hostname(); herr == nil && host != "" {
		name += " (" + host + ")"
	}
	tmpl := &x509.Certificate{
		SerialNumber:          randomSerial(),
		Subject:               pkix.Name{Organization: []string{"go-trmnl"}, CommonName: name},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("tlscert: create CA: %w", err)
	}
	return encodePair(der, key)
}

func issueServer(ca *x509.Certificate, caKey crypto.Signer, hosts []string, now time.Time) (certPEM, keyPEM []byte, err error) {
	if len(hosts) == 0 {
		return nil, nil, errors.New("tlscert: no hosts for the server certificate")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: randomSerial(),
		Subject:      pkix.Name{Organization: []string{"go-trmnl"}, CommonName: hosts[0]},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(serverValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("tlscert: issue server certificate: %w", err)
	}
	return encodePair(der, key)
}

func parseCA(certPEM, keyPEM []byte) (*x509.Certificate, crypto.Signer, error) {
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("tlscert: load local CA: %w", err)
	}
	ca, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, nil, fmt.Errorf("tlscert: parse local CA: %w", err)
	}
	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok || !ca.IsCA {
		return nil, nil, errors.New("tlscert: local CA certificate/key is not usable as a CA")
	}
	return ca, signer, nil
}

func encodePair(der []byte, key *ecdsa.PrivateKey) (certPEM, keyPEM []byte, err error) {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

func randomSerial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return n
}

// writeFile atomically replaces path with data at the given mode.
func writeFile(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("tlscert: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(mode)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("tlscert: write %s: %w", path, err)
	}
	return nil
}

func newestModTime(paths ...string) (time.Time, error) {
	var newest time.Time
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return time.Time{}, err
		}
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest, nil
}

// normalizeHosts lowercases, trims and de-duplicates hosts, dropping empties
// and any port suffix, while keeping the first occurrence order.
func normalizeHosts(hosts []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if host, _, err := net.SplitHostPort(h); err == nil {
			h = host
		}
		h = strings.Trim(h, "[]")
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// DefaultHosts returns the names a local CA certificate should cover by
// default: this machine's hostname (plus its mDNS ".local" form), the host of
// the public base URL, and loopback.
func DefaultHosts(baseURLHost string) []string {
	var hosts []string
	if h, err := os.Hostname(); err == nil && h != "" {
		hosts = append(hosts, h)
		if !strings.Contains(h, ".") {
			hosts = append(hosts, h+".local")
		}
	}
	hosts = append(hosts, baseURLHost, "localhost", "127.0.0.1", "::1")
	return normalizeHosts(hosts)
}
