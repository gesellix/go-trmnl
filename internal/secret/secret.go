// Package secret provides optional symmetric encryption for sensitive values
// (e.g. OAuth refresh tokens, CalDAV passwords) stored in the local database. A
// key is derived from TRMNL_SECRET_KEY; when it is unset, encryption is disabled
// and values are stored as-is.
//
// Stored values are tagged with a version:
//
//	enc:v2:  AES-256-GCM, key derived with scrypt (current; used for writes)
//	enc:v1:  AES-256-GCM, key derived with SHA-256 (legacy; read-only)
//	(none)   plaintext (no key was configured when written)
//
// The legacy reader lets existing data keep working and be migrated to v2; new
// data is always written as v2.
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

	"golang.org/x/crypto/scrypt"
)

const (
	legacyPrefix  = "enc:v1:" // AES-GCM with a SHA-256-derived key
	currentPrefix = "enc:v2:" // AES-GCM with a scrypt-derived key
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
	current cipher.AEAD // v2 (scrypt): writes and reads
	legacy  cipher.AEAD // v1 (SHA-256): read-only, for migration
	enabled bool
}

// NewFromEnv builds a Box from TRMNL_SECRET_KEY (disabled when empty).
func NewFromEnv() *Box { return New(os.Getenv("TRMNL_SECRET_KEY")) }

// New builds a Box from a key string. An empty key yields a disabled Box.
func New(key string) *Box {
	if key == "" {
		return &Box{}
	}
	current := aeadFromKey(scryptKey(key))
	legacy := aeadFromKey(sha256Key(key))
	if current == nil || legacy == nil {
		return &Box{}
	}
	return &Box{current: current, legacy: legacy, enabled: true}
}

func scryptKey(key string) []byte {
	k, err := scrypt.Key([]byte(key), scryptSalt, scryptN, scryptR, scryptP, 32)
	if err != nil {
		return nil
	}
	return k
}

// sha256Key derives the legacy (v1) key. Retained only to read and migrate
// data written before v2; new data uses scrypt.
func sha256Key(key string) []byte {
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

func aeadFromKey(k []byte) cipher.AEAD {
	if k == nil {
		return nil
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil
	}
	return gcm
}

// Enabled reports whether a key is configured.
func (b *Box) Enabled() bool { return b != nil && b.enabled }

// Encrypt returns a tagged v2 ciphertext when enabled, or the input unchanged
// when disabled (or empty).
func (b *Box) Encrypt(s string) (string, error) {
	if !b.Enabled() || s == "" {
		return s, nil
	}
	return seal(b.current, currentPrefix, s)
}

// Decrypt reverses Encrypt, dispatching on the version tag. Untagged (plaintext)
// values are returned unchanged; a tagged value requires an enabled box.
func (b *Box) Decrypt(s string) (string, error) {
	switch {
	case strings.HasPrefix(s, currentPrefix):
		return b.open(b.current, currentPrefix, s)
	case strings.HasPrefix(s, legacyPrefix):
		return b.open(b.legacy, legacyPrefix, s)
	default:
		return s, nil // plaintext
	}
}

// NeedsUpgrade reports whether a stored value should be re-encrypted to the
// current (v2) format: it is legacy (v1), or plaintext while a key is set.
func (b *Box) NeedsUpgrade(stored string) bool {
	if !b.Enabled() || stored == "" {
		return false
	}
	if strings.Contains(stored, legacyPrefix) {
		return true
	}
	return !strings.Contains(stored, currentPrefix)
}

func seal(aead cipher.AEAD, prefix, s string) (string, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nonce, nonce, []byte(s), nil)
	return prefix + base64.StdEncoding.EncodeToString(ct), nil
}

func (b *Box) open(aead cipher.AEAD, prefix, s string) (string, error) {
	if !b.Enabled() || aead == nil {
		return "", errors.New("secret: value is encrypted but TRMNL_SECRET_KEY is not set")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, prefix))
	if err != nil {
		return "", err
	}
	ns := aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("secret: ciphertext too short")
	}
	pt, err := aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
