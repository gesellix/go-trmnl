// Package device holds device domain logic: provisioning new devices
// (generating credentials) and normalizing identifiers. It depends only on the
// store package.
package device

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/gesellix/go-trmnl/internal/store"
)

// NormalizeMAC validates and canonicalizes a MAC address from the ID header to
// upper-case colon-separated form (e.g. "AA:BB:CC:DD:EE:FF").
func NormalizeMAC(raw string) (string, error) {
	hw, err := net.ParseMAC(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid MAC %q: %w", raw, err)
	}
	return strings.ToUpper(hw.String()), nil
}

// Provision returns the device for mac, creating it (with freshly generated
// credentials) if it does not yet exist. The returned bool reports whether a
// new device was created.
func Provision(st *store.Store, mac, model, fw string) (*store.Device, bool, error) {
	if d, err := st.GetDeviceByMAC(mac); err == nil {
		return d, false, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, false, err
	}

	// Create, retrying on the (extremely unlikely) friendly_id/api_key clash.
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		d := &store.Device{
			MAC:        mac,
			APIKey:     generateToken(22),
			FriendlyID: generateFriendlyID(),
			Model:      nullStr(model),
			FWVersion:  nullStr(fw),
		}
		created, err := st.CreateDevice(d)
		if err == nil {
			return created, true, nil
		}
		// A concurrent setup for the same MAC may have won the race.
		if existing, getErr := st.GetDeviceByMAC(mac); getErr == nil {
			return existing, false, nil
		}
		lastErr = err
	}
	return nil, false, fmt.Errorf("provision device: %w", lastErr)
}

const (
	tokenAlphabet      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	friendlyIDAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // Crockford base32
	friendlyIDLength   = 6
)

// generateToken returns a random URL-safe token of n characters.
func generateToken(n int) string { return randString(n, tokenAlphabet) }

// generateFriendlyID returns a short human-friendly device identifier.
func generateFriendlyID() string { return randString(friendlyIDLength, friendlyIDAlphabet) }

func randString(n int, alphabet string) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail; panic is acceptable at startup paths.
		panic(fmt.Sprintf("device: read random: %v", err))
	}
	out := make([]byte, n)
	for i := range b {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out)
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
