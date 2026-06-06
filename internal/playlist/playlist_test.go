package playlist_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/gesellix/go-trmnl/internal/playlist"
	"github.com/gesellix/go-trmnl/internal/store"
)

func TestNextScreenRoundRobin(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	pl, _ := st.CreatePlaylist("default")
	var screenIDs []int64
	for _, name := range []string{"a", "b", "c"} {
		p, _ := st.CreatePlugin("clock", name)
		sc, _ := st.CreateScreen(p.ID, name, "{}")
		st.AddPlaylistItem(pl.ID, sc.ID)
		screenIDs = append(screenIDs, sc.ID)
	}

	d, _ := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:01", APIKey: "k", FriendlyID: "F1"})
	if err := st.UpdateDeviceSettings(d.ID, "", 900, sql.NullInt64{Int64: pl.ID, Valid: true}); err != nil {
		t.Fatal(err)
	}

	// Six picks should cycle a, b, c, a, b, c.
	want := []int64{screenIDs[0], screenIDs[1], screenIDs[2], screenIDs[0], screenIDs[1], screenIDs[2]}
	for i, wantID := range want {
		d, _ = st.GetDeviceByID(d.ID) // reload to pick up advanced cursor
		sc, err := playlist.NextScreen(st, d)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		if sc.ID != wantID {
			t.Errorf("pick %d: screen %d, want %d", i, sc.ID, wantID)
		}
	}
}

func TestNextScreenNoPlaylist(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	d, _ := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:02", APIKey: "k", FriendlyID: "F2"})
	if _, err := playlist.NextScreen(st, d); err != playlist.ErrNoScreen {
		t.Fatalf("err = %v, want ErrNoScreen", err)
	}
}
