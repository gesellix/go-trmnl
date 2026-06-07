package secret

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")

	key, created, err := LoadOrCreateKey(path)
	if err != nil || !created || key == "" {
		t.Fatalf("first call: key=%q created=%v err=%v", key, created, err)
	}
	if fi, err := os.Stat(path); err != nil {
		t.Fatalf("key file not written: %v", err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Errorf("key file mode = %v, want 0600", fi.Mode().Perm())
	}

	// Second call returns the same key and does not recreate it.
	key2, created2, err := LoadOrCreateKey(path)
	if err != nil || created2 || key2 != key {
		t.Errorf("second call: key=%q created=%v err=%v, want stable + created=false", key2, created2, err)
	}

	// The generated key drives a working box.
	if got, _ := New(key).Decrypt(mustEncrypt(t, New(key), "x")); got != "x" {
		t.Errorf("generated key does not round-trip: %q", got)
	}
}

func mustEncrypt(t *testing.T, b *Box, s string) string {
	t.Helper()
	ct, err := b.Encrypt(s)
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

func TestRoundTripV2(t *testing.T) {
	b := New("a-passphrase")
	if !b.Enabled() {
		t.Fatal("box should be enabled with a key")
	}
	ct, err := b.Encrypt("refresh-token-123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ct, currentPrefix) {
		t.Fatalf("ciphertext not tagged v2: %q", ct)
	}
	got, err := b.Decrypt(ct)
	if err != nil || got != "refresh-token-123" {
		t.Errorf("round trip = %q err=%v", got, err)
	}
}

func TestLegacyV1Rejected(t *testing.T) {
	// v1 is no longer decryptable; it must surface as a clear error rather than
	// being treated as plaintext.
	if _, err := New("k").Decrypt(legacyPrefix + "anything"); err == nil {
		t.Error("expected an error decrypting a legacy v1 value")
	}
}

func TestNeedsUpgrade(t *testing.T) {
	b := New("k")
	v2, _ := b.Encrypt("x")

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"plaintext", "plain-secret", true},
		{"current v2", v2, false},
		{"legacy v1 (cannot upgrade)", legacyPrefix + "abc", false},
		{"empty", "", false},
		{"json blob with v2 field", `{"email":"a@b","refresh_token":"` + v2 + `"}`, false},
	}
	for _, c := range cases {
		if got := b.NeedsUpgrade(c.in); got != c.want {
			t.Errorf("%s: NeedsUpgrade = %v, want %v", c.name, got, c.want)
		}
	}
	if New("").NeedsUpgrade("plain-secret") {
		t.Error("disabled box should not request an upgrade")
	}
}

func TestEmptyValuePassthrough(t *testing.T) {
	if ct, _ := New("k").Encrypt(""); ct != "" {
		t.Errorf("empty encrypt = %q, want empty", ct)
	}
}

func TestDisabledBoxPassesThrough(t *testing.T) {
	var b *Box // nil is a valid disabled box
	if b.Enabled() {
		t.Fatal("nil box should be disabled")
	}
	if ct, err := b.Encrypt("secret"); err != nil || ct != "secret" {
		t.Errorf("disabled encrypt = %q err=%v, want passthrough", ct, err)
	}
	if got, err := b.Decrypt("plain"); err != nil || got != "plain" {
		t.Errorf("legacy plaintext decrypt = %q err=%v", got, err)
	}
}

func TestDecryptV2WithoutKeyErrors(t *testing.T) {
	ct, _ := New("k").Encrypt("secret")
	if _, err := New("").Decrypt(ct); err == nil {
		t.Error("expected error decrypting a v2 value without a key")
	}
}

func TestWrongKeyFails(t *testing.T) {
	ct, _ := New("right").Encrypt("secret")
	if _, err := New("wrong").Decrypt(ct); err == nil {
		t.Error("expected error with wrong key")
	}
}
