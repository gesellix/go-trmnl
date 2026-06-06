package main

import (
	"github.com/gesellix/go-trmnl/internal/store"
)

// seedExample populates a fresh database with an example playlist containing a
// clock and a weather screen, so a newly provisioned device shows something
// useful out of the box. It is a no-op once any screens or playlists exist.
func seedExample(st *store.Store) error {
	pls, err := st.ListPlaylists()
	if err != nil {
		return err
	}
	scrs, err := st.ListScreens()
	if err != nil {
		return err
	}
	if len(pls) > 0 || len(scrs) > 0 {
		return nil
	}

	pl, err := st.CreatePlaylist("Example")
	if err != nil {
		return err
	}

	type seed struct {
		typ, name, settings string
	}
	for _, s := range []seed{
		{"clock", "Clock", `{"use_24h":true,"label":"go-trmnl"}`},
		{"weather", "Weather (Berlin)", `{"location":"Berlin","units":"metric","label":"Weather"}`},
	} {
		pg, err := st.CreatePlugin(s.typ, s.name)
		if err != nil {
			return err
		}
		sc, err := st.CreateScreen(pg.ID, s.name, s.settings)
		if err != nil {
			return err
		}
		if err := st.AddPlaylistItem(pl.ID, sc.ID); err != nil {
			return err
		}
	}
	return nil
}
