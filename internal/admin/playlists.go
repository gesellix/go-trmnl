package admin

import (
	"net/http"
)

// PlaylistsList shows all playlists and the create form.
func (h *Handler) PlaylistsList(w http.ResponseWriter, r *http.Request) {
	pls, _ := h.store.ListPlaylists()
	h.render(w, "playlists", map[string]any{"Nav": "playlists", "Playlists": pls})
}

// PlaylistCreate creates a playlist.
func (h *Handler) PlaylistCreate(w http.ResponseWriter, r *http.Request) {
	name := def("Playlist", r.FormValue("name"))
	pl, err := h.store.CreatePlaylist(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/playlists/"+i64(pl.ID), http.StatusFound)
}

// PlaylistDetail shows a playlist's items and the add-screen form.
func (h *Handler) PlaylistDetail(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pl, err := h.store.GetPlaylist(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	items, _ := h.store.ListPlaylistItems(id)

	// Resolve item screen names for display.
	type itemView struct {
		ItemID     int64
		ScreenID   int64
		ScreenName string
		Position   int
	}
	views := make([]itemView, 0, len(items))
	for _, it := range items {
		name := "(deleted screen)"
		if sc, err := h.store.GetScreen(it.ScreenID); err == nil {
			name = sc.Name
		}
		views = append(views, itemView{ItemID: it.ID, ScreenID: it.ScreenID, ScreenName: name, Position: it.Position})
	}

	allScreens, _ := h.store.ListScreens()
	h.render(w, "playlist", map[string]any{
		"Nav":        "playlists",
		"Playlist":   pl,
		"Items":      views,
		"AllScreens": allScreens,
	})
}

// PlaylistAddItem appends a screen to the playlist.
func (h *Handler) PlaylistAddItem(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	screenID, ok := parseInt64(r.FormValue("screen_id"))
	if !ok {
		http.Error(w, "missing screen_id", http.StatusBadRequest)
		return
	}
	_ = h.store.AddPlaylistItem(id, screenID)
	http.Redirect(w, r, "/admin/playlists/"+chiID(r), http.StatusFound)
}

// PlaylistRemoveItem removes an item from the playlist.
func (h *Handler) PlaylistRemoveItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := idParam(r, "itemID")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.store.RemovePlaylistItem(itemID)
	http.Redirect(w, r, "/admin/playlists/"+chiID(r), http.StatusFound)
}

// PlaylistDelete removes a playlist.
func (h *Handler) PlaylistDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.store.DeletePlaylist(id)
	http.Redirect(w, r, "/admin/playlists", http.StatusFound)
}
