// Package secret provides optional symmetric encryption for sensitive values
// (e.g. OAuth refresh tokens) stored in the local database. A key is derived
// from TRMNL_SECRET_KEY; when it is unset, encryption is disabled and values
// are stored as-is. The on-disk format is tagged so that disabling the key
// still leaves previously written plaintext readable, and a present key can
// read both plaintext (legacy) and encrypted values.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
)

// prefix tags an encrypted, base64-encoded value.
const prefix = "enc:v1:"

// Box encrypts and decrypts strings. The zero value (and a nil *Box) is a valid,
// disabled box that passes values through unchanged.
type Box struct {
	gcm     cipher.AEAD
	enabled bool
}

// NewFromEnv builds a Box from TRMNL_SECRET_KEY (disabled when empty).
func NewFromEnv() *Box { return New(os.Getenv("TRMNL_SECRET_KEY")) }

// New builds a Box from a key string. An empty key yields a disabled Box. The
// key may be any length; it is hashed to a 32-byte AES-256 key.
func New(key string) *Box {
	if key == "" {
		return &Box{}
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return &Box{}
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return &Box{}
	}
	return &Box{gcm: gcm, enabled: true}
}

// Enabled reports whether a key is configured.
func (b *Box) Enabled() bool { return b != nil && b.enabled }

// Encrypt returns a tagged ciphertext when enabled, or the input unchanged when
// disabled (or empty).
func (b *Box) Encrypt(s string) (string, error) {
	if !b.Enabled() || s == "" {
		return s, nil
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := b.gcm.Seal(nonce, nonce, []byte(s), nil)
	return prefix + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. Untagged (legacy plaintext) values are returned
// unchanged. A tagged value requires an enabled box.
func (b *Box) Decrypt(s string) (string, error) {
	if !strings.HasPrefix(s, prefix) {
		return s, nil // legacy plaintext
	}
	if !b.Enabled() {
		return "", errors.New("secret: value is encrypted but TRMNL_SECRET_KEY is not set")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, prefix))
	if err != nil {
		return "", err
	}
	ns := b.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("secret: ciphertext too short")
	}
	pt, err := b.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
