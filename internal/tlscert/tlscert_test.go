package tlscert

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func leafOf(t *testing.T, s *Source) *x509.Certificate {
	t.Helper()
	cert, err := s.certificate()
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return leaf
}

func TestLocalCAServesTrustedCertificate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tls")
	src := NewLocalCA(dir, []string{"trmnl.fritz.box", "127.0.0.1", "TRMNL.fritz.box:8443", ""})

	cfg, err := src.TLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got := src.Hosts(); len(got) != 2 {
		t.Errorf("hosts not normalized/deduplicated: %v", got)
	}

	for name, mode := range map[string]os.FileMode{caKeyFile: 0o600, serverKeyFile: 0o600, caCertFile: 0o644} {
		fi, serr := os.Stat(filepath.Join(dir, name))
		if serr != nil {
			t.Fatal(serr)
		}
		if fi.Mode().Perm() != mode {
			t.Errorf("%s mode = %o, want %o", name, fi.Mode().Perm(), mode)
		}
	}

	// A client trusting only the CA certificate completes a handshake for the
	// configured hostname, as a browser would once the CA is installed.
	caPEM, err := src.CACertPEM()
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		t.Fatal("CA PEM not parseable")
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	srv.TLS = cfg
	srv.StartTLS()
	defer srv.Close()

	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "trmnl.fritz.box"},
	}}
	res, err := client.Get("https://127.0.0.1:" + port + "/")
	if err != nil {
		t.Fatalf("TLS request with the local CA failed: %v", err)
	}
	_ = res.Body.Close()

	fp, err := Fingerprint(caPEM)
	if err != nil || len(strings.Split(fp, ":")) != 32 {
		t.Errorf("fingerprint = %q, %v", fp, err)
	}
}

func TestLocalCAReusesAndRenews(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	src := NewLocalCA(dir, []string{"trmnl.fritz.box"})
	src.now = func() time.Time { return now }
	first := leafOf(t, src)
	caBefore, _ := os.ReadFile(filepath.Join(dir, caCertFile))

	// A restart within the validity window reuses the stored certificate.
	again := NewLocalCA(dir, []string{"trmnl.fritz.box"})
	again.now = func() time.Time { return now.Add(24 * time.Hour) }
	if leafOf(t, again).SerialNumber.Cmp(first.SerialNumber) != 0 {
		t.Error("certificate was re-issued although still valid")
	}

	// A new host triggers a re-issue from the same CA.
	more := NewLocalCA(dir, []string{"trmnl.fritz.box", "pi.home.arpa"})
	more.now = func() time.Time { return now.Add(48 * time.Hour) }
	leaf := leafOf(t, more)
	if leaf.SerialNumber.Cmp(first.SerialNumber) == 0 || leaf.VerifyHostname("pi.home.arpa") != nil {
		t.Error("certificate was not re-issued for a new host")
	}
	caAfter, _ := os.ReadFile(filepath.Join(dir, caCertFile))
	if string(caBefore) != string(caAfter) {
		t.Error("CA must not change when re-issuing the server certificate")
	}

	// A running server renews shortly before expiry, after the recheck interval.
	late := leaf.NotAfter.Add(-renewBefore + time.Hour)
	more.now = func() time.Time { return late }
	if renewed := leafOf(t, more); renewed.SerialNumber.Cmp(leaf.SerialNumber) == 0 || !renewed.NotAfter.After(late.Add(renewBefore)) {
		t.Error("certificate close to expiry was not renewed")
	}
}

func TestLocalCARefusesPartialCA(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, caCertFile), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalCA(dir, []string{"localhost"}).TLSConfig(); err == nil {
		t.Fatal("expected an error when only the CA certificate exists")
	}
}

func TestFromFilesReloadsOnChange(t *testing.T) {
	// Produce two distinct certificate/key pairs via the local CA.
	ca := NewLocalCA(t.TempDir(), []string{"a.example.com"})
	pair1 := leafOf(t, ca)
	certPEM1, _ := os.ReadFile(filepath.Join(ca.dir, serverCertFile))
	keyPEM1, _ := os.ReadFile(filepath.Join(ca.dir, serverKeyFile))

	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	_ = os.WriteFile(certFile, certPEM1, 0o644)
	_ = os.WriteFile(keyFile, keyPEM1, 0o600)

	now := time.Now()
	src := NewFromFiles(certFile, keyFile)
	src.now = func() time.Time { return now }
	if _, err := src.TLSConfig(); err != nil {
		t.Fatal(err)
	}
	if leafOf(t, src).SerialNumber.Cmp(pair1.SerialNumber) != 0 {
		t.Fatal("unexpected certificate")
	}
	if _, err := src.CACertPEM(); err == nil {
		t.Error("file mode has no local CA")
	}

	other := NewLocalCA(t.TempDir(), []string{"b.example.com"})
	pair2 := leafOf(t, other)
	certPEM2, _ := os.ReadFile(filepath.Join(other.dir, serverCertFile))
	keyPEM2, _ := os.ReadFile(filepath.Join(other.dir, serverKeyFile))
	_ = os.WriteFile(certFile, certPEM2, 0o644)
	_ = os.WriteFile(keyFile, keyPEM2, 0o600)
	future := now.Add(2 * recheckEvery)
	_ = os.Chtimes(certFile, future, future)
	src.now = func() time.Time { return future }
	if leafOf(t, src).SerialNumber.Cmp(pair2.SerialNumber) != 0 {
		t.Error("changed certificate files were not reloaded")
	}

	// A broken replacement keeps the previous certificate in service.
	_ = os.WriteFile(certFile, []byte("broken"), 0o644)
	later := future.Add(2 * recheckEvery)
	_ = os.Chtimes(certFile, later, later)
	src.now = func() time.Time { return later }
	if leafOf(t, src).SerialNumber.Cmp(pair2.SerialNumber) != 0 {
		t.Error("broken replacement should keep serving the previous certificate")
	}
}

func TestDefaultHosts(t *testing.T) {
	hosts := DefaultHosts("192.168.1.10")
	for _, want := range []string{"192.168.1.10", "localhost", "127.0.0.1", "::1"} {
		found := false
		for _, h := range hosts {
			found = found || h == want
		}
		if !found {
			t.Errorf("DefaultHosts missing %q: %v", want, hosts)
		}
	}
}
