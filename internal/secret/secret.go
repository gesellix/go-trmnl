// Package secret provides optional symmetric encryption for sensitive values
// (e.g. OAuth refresh tokens, CalDAV passwords) stored in the local database. A
// key is derived from TRMNL_SECRET_KEY with scrypt; when it is unset, encryption
// is disabled and values are stored as-is.
//
// Stored values are tagged:
//
//	enc:v2:  AES-256-GCM, key derived with scrypt (current)
//	(none)   plaintext (no key was configured when written)
//
// An older format, enc:v1: (AES-GCM with a SHA-256-derived key), is no longer
// supported; data written by that scheme must have been migrated to v2 by a
// prior release. A leftover v1 value decrypts to a clear error.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/scrypt"
)

const (
	legacyPrefix  = "enc:v1:" // pre-scrypt; recognized only to report a clear error
	currentPrefix = "enc:v2:" // AES-256-GCM with a scrypt-derived key
)

// scryptSalt is a fixed application salt. The configured key is itself the
// secret; a fixed salt keeps derivation deterministic so stored ciphertext
// stays decryptable across restarts. scrypt's cost is what hardens a weak key.
var scryptSalt = []byte("go-trmnl/secret/scrypt/v2")

const (
	scryptN = 1 << 15 // CPU/memory cost
	scryptR = 8
	scryptP = 1
)

// Box encrypts and decrypts strings. The zero value (and a nil *Box) is a valid,
// disabled box that passes values through unchanged.
type Box struct {
	aead    cipher.AEAD
	enabled bool
}

// NewFromEnv builds a Box from TRMNL_SECRET_KEY (disabled when empty).
func NewFromEnv() *Box { return New(os.Getenv("TRMNL_SECRET_KEY")) }

// New builds a Box from a key string. An empty key yields a disabled Box.
func New(key string) *Box {
	if key == "" {
		return &Box{}
	}
	k, err := scrypt.Key([]byte(key), scryptSalt, scryptN, scryptR, scryptP, 32)
	if err != nil {
		return &Box{}
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return &Box{}
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return &Box{}
	}
	return &Box{aead: gcm, enabled: true}
}

// LoadOrCreateKey returns the key stored at path, creating it with a fresh
// random value (file mode 0600) when absent or empty. This lets encryption be
// on by default: the operator does not have to manage a key, while an explicit
// TRMNL_SECRET_KEY still takes precedence. Note the key sits next to the
// database, so it mainly protects the database file in isolation (e.g. a stray
// copy or backup), not a full data-directory compromise.
func LoadOrCreateKey(path string) (key string, created bool, err error) {
	if b, rerr := os.ReadFile(path); rerr == nil {
		if k := strings.TrimSpace(string(b)); k != "" {
			return k, false, nil
		}
	}
	buf := make([]byte, 32)
	if _, rerr := io.ReadFull(rand.Reader, buf); rerr != nil {
		return "", false, rerr
	}
	key = base64.StdEncoding.EncodeToString(buf)
	if werr := os.WriteFile(path, []byte(key+"\n"), 0o600); werr != nil {
		return "", false, werr
	}
	return key, true, nil
}

// Enabled reports whether a key is configured.
func (b *Box) Enabled() bool { return b != nil && b.enabled }

// Encrypt returns a tagged v2 ciphertext when enabled, or the input unchanged
// when disabled (or empty).
func (b *Box) Encrypt(s string) (string, error) {
	if !b.Enabled() || s == "" {
		return s, nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := b.aead.Seal(nonce, nonce, []byte(s), nil)
	return currentPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. Untagged (plaintext) values are returned unchanged;
// a v2 value requires an enabled box; a legacy v1 value is rejected with a
// clear error.
func (b *Box) Decrypt(s string) (string, error) {
	switch {
	case strings.HasPrefix(s, currentPrefix):
		if !b.Enabled() {
			return "", errors.New("secret: value is encrypted but TRMNL_SECRET_KEY is not set")
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, currentPrefix))
		if err != nil {
			return "", err
		}
		ns := b.aead.NonceSize()
		if len(raw) < ns {
			return "", errors.New("secret: ciphertext too short")
		}
		pt, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
		if err != nil {
			return "", err
		}
		return string(pt), nil
	case strings.HasPrefix(s, legacyPrefix):
		return "", errors.New("secret: enc:v1: values are no longer supported; re-enter the affected credential")
	default:
		return s, nil // plaintext
	}
}

// NeedsUpgrade reports whether a stored value should be encrypted to the current
// format. Only plaintext qualifies: already-current (v2) values are left alone,
// and legacy v1 values can no longer be re-encrypted (they surface as a Decrypt
// error when used).
func (b *Box) NeedsUpgrade(stored string) bool {
	if !b.Enabled() || stored == "" {
		return false
	}
	if strings.Contains(stored, currentPrefix) || strings.Contains(stored, legacyPrefix) {
		return false
	}
	return true
}
