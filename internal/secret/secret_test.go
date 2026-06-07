package secret

import "testing"

func TestRoundTrip(t *testing.T) {
	b := New("a-passphrase")
	if !b.Enabled() {
		t.Fatal("box should be enabled with a key")
	}
	ct, err := b.Encrypt("refresh-token-123")
	if err != nil {
		t.Fatal(err)
	}
	if ct == "refresh-token-123" {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := b.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if got != "refresh-token-123" {
		t.Errorf("round trip = %q", got)
	}
}

func TestEmptyValuePassthrough(t *testing.T) {
	b := New("k")
	if ct, _ := b.Encrypt(""); ct != "" {
		t.Errorf("empty encrypt = %q, want empty", ct)
	}
}

func TestDisabledBoxPassesThrough(t *testing.T) {
	var b *Box // nil is a valid disabled box
	if b.Enabled() {
		t.Fatal("nil box should be disabled")
	}
	ct, err := b.Encrypt("secret")
	if err != nil || ct != "secret" {
		t.Errorf("disabled encrypt = %q err=%v, want passthrough", ct, err)
	}
	// Legacy plaintext decrypts to itself.
	if got, err := b.Decrypt("plain"); err != nil || got != "plain" {
		t.Errorf("legacy decrypt = %q err=%v", got, err)
	}
}

func TestDecryptEncryptedWithoutKeyErrors(t *testing.T) {
	ct, _ := New("k").Encrypt("secret")
	if _, err := New("").Decrypt(ct); err == nil {
		t.Error("expected error decrypting without a key")
	}
}

func TestWrongKeyFails(t *testing.T) {
	ct, _ := New("right").Encrypt("secret")
	if _, err := New("wrong").Decrypt(ct); err == nil {
		t.Error("expected error with wrong key")
	}
}
