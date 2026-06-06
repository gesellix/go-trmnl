// Package playlist selects the next screen to show on a device, advancing a
// per-device round-robin cursor.
package playlist

import (
	"errors"

	"github.com/gesellix/go-trmnl/internal/store"
)

// ErrNoScreen is returned when a device has no playlist or its playlist has no
// visible items.
var ErrNoScreen = errors.New("no screen available")

// NextScreen returns the next visible screen for the device and advances the
// device's playlist cursor. It returns ErrNoScreen when nothing is assigned.
func NextScreen(st *store.Store, d *store.Device) (*store.Screen, error) {
	if !d.PlaylistID.Valid {
		return nil, ErrNoScreen
	}
	items, err := st.ListPlaylistItems(d.PlaylistID.Int64)
	if err != nil {
		return nil, err
	}
	visible := items[:0]
	for _, it := range items {
		if it.Visible {
			visible = append(visible, it)
		}
	}
	if len(visible) == 0 {
		return nil, ErrNoScreen
	}

	idx := d.PlaylistCursor % len(visible)
	if idx < 0 {
		idx = 0
	}
	chosen := visible[idx]

	// Advance the cursor for the next poll; best-effort persistence.
	_ = st.SetPlaylistCursor(d.ID, (idx+1)%len(visible))

	return st.GetScreen(chosen.ScreenID)
}
