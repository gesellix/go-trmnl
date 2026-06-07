package secret

import (
	"strings"
	"testing"
)

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

func TestReadsLegacyV1(t *testing.T) {
	const key = "master"
	// Reproduce a v1 value (AES-GCM with a SHA-256-derived key).
	v1, err := seal(aeadFromKey(sha256Key(key)), legacyPrefix, "legacy-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v1, legacyPrefix) {
		t.Fatalf("not a v1 value: %q", v1)
	}
	got, err := New(key).Decrypt(v1)
	if err != nil || got != "legacy-secret" {
		t.Errorf("legacy decrypt = %q err=%v, want legacy-secret", got, err)
	}
}

func TestNeedsUpgrade(t *testing.T) {
	b := New("k")
	v1, _ := seal(aeadFromKey(sha256Key("k")), legacyPrefix, "x")
	v2, _ := b.Encrypt("x")

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"legacy v1", v1, true},
		{"current v2", v2, false},
		{"plaintext", "plain-secret", true},
		{"empty", "", false},
		{"json blob with v2 field", `{"email":"a@b","refresh_token":"` + v2 + `"}`, false},
		{"json blob with v1 field", `{"email":"a@b","refresh_token":"` + v1 + `"}`, true},
	}
	for _, c := range cases {
		if got := b.NeedsUpgrade(c.in); got != c.want {
			t.Errorf("%s: NeedsUpgrade = %v, want %v", c.name, got, c.want)
		}
	}
	// A disabled box never asks to upgrade.
	if New("").NeedsUpgrade(v1) {
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

func TestDecryptEncryptedWithoutKeyErrors(t *testing.T) {
	v2, _ := New("k").Encrypt("secret")
	v1, _ := seal(aeadFromKey(sha256Key("k")), legacyPrefix, "secret")
	for _, ct := range []string{v2, v1} {
		if _, err := New("").Decrypt(ct); err == nil {
			t.Errorf("expected error decrypting %q without a key", ct[:7])
		}
	}
}

func TestWrongKeyFails(t *testing.T) {
	ct, _ := New("right").Encrypt("secret")
	if _, err := New("wrong").Decrypt(ct); err == nil {
		t.Error("expected error with wrong key")
	}
}
