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
	screenID, idx, n, err := pick(st, d)
	if err != nil {
		return nil, err
	}
	// Advance the cursor for the next poll; best-effort persistence.
	_ = st.SetPlaylistCursor(d.ID, (idx+1)%n)

	return st.GetScreen(screenID)
}

// PeekScreen returns the screen the device's next poll will show, without
// advancing the cursor. It returns ErrNoScreen when nothing is assigned.
func PeekScreen(st *store.Store, d *store.Device) (*store.Screen, error) {
	screenID, _, _, err := pick(st, d)
	if err != nil {
		return nil, err
	}
	return st.GetScreen(screenID)
}

// pick selects the visible playlist item at the device's cursor. It returns
// the item's screen ID, its index among the visible items, and their count.
func pick(st *store.Store, d *store.Device) (screenID int64, idx, n int, err error) {
	if !d.PlaylistID.Valid {
		return 0, 0, 0, ErrNoScreen
	}
	items, err := st.ListPlaylistItems(d.PlaylistID.Int64)
	if err != nil {
		return 0, 0, 0, err
	}
	visible := items[:0]
	for _, it := range items {
		if it.Visible {
			visible = append(visible, it)
		}
	}
	if len(visible) == 0 {
		return 0, 0, 0, ErrNoScreen
	}

	idx = d.PlaylistCursor % len(visible)
	if idx < 0 {
		idx = 0
	}
	return visible[idx].ScreenID, idx, len(visible), nil
}
