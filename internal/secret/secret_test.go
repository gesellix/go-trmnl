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
