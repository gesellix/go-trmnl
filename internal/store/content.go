package store

import (
	"database/sql"
	"errors"
)

// --- Plugins ---

// CreatePlugin inserts a plugin instance.
func (s *Store) CreatePlugin(typ, name string) (*Plugin, error) {
	res, err := s.db.Exec(`INSERT INTO plugins (type, name, created_at) VALUES (?, ?, unixepoch())`, typ, name)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetPlugin(id)
}

// GetPlugin returns a plugin instance by ID.
func (s *Store) GetPlugin(id int64) (*Plugin, error) {
	var p Plugin
	err := s.db.QueryRow(`SELECT id, type, name, created_at FROM plugins WHERE id = ?`, id).
		Scan(&p.ID, &p.Type, &p.Name, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// --- Screens ---

const screenColumns = `id, plugin_id, name, settings_json, rendered_hash, rendered_at, created_at`

func scanScreen(row interface{ Scan(...any) error }) (*Screen, error) {
	var sc Screen
	err := row.Scan(&sc.ID, &sc.PluginID, &sc.Name, &sc.SettingsJSON, &sc.RenderedHash, &sc.RenderedAt, &sc.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

// CreateScreen inserts a screen bound to a plugin instance.
func (s *Store) CreateScreen(pluginID int64, name, settingsJSON string) (*Screen, error) {
	if settingsJSON == "" {
		settingsJSON = "{}"
	}
	res, err := s.db.Exec(`INSERT INTO screens (plugin_id, name, settings_json, created_at)
		VALUES (?, ?, ?, unixepoch())`, pluginID, name, settingsJSON)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetScreen(id)
}

// GetScreen returns a screen by ID.
func (s *Store) GetScreen(id int64) (*Screen, error) {
	return scanScreen(s.db.QueryRow(`SELECT `+screenColumns+` FROM screens WHERE id = ?`, id))
}

// ListScreens returns all screens, newest first.
func (s *Store) ListScreens() ([]*Screen, error) {
	rows, err := s.db.Query(`SELECT ` + screenColumns + ` FROM screens ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Screen
	for rows.Next() {
		sc, err := scanScreen(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// UpdateScreenSettings updates a screen's name and settings JSON, clearing the
// rendered cache pointer so it re-renders.
func (s *Store) UpdateScreenSettings(id int64, name, settingsJSON string) error {
	if settingsJSON == "" {
		settingsJSON = "{}"
	}
	_, err := s.db.Exec(`UPDATE screens SET name = ?, settings_json = ?, rendered_hash = NULL, rendered_at = NULL WHERE id = ?`,
		name, settingsJSON, id)
	return err
}

// SetScreenRendered records the content hash and time of the latest render.
func (s *Store) SetScreenRendered(id int64, hash string) error {
	_, err := s.db.Exec(`UPDATE screens SET rendered_hash = ?, rendered_at = unixepoch() WHERE id = ?`, hash, id)
	return err
}

// ActiveRenderHashes returns the distinct, currently-referenced render hashes
// across all screens, used to decide which cached image files to keep.
func (s *Store) ActiveRenderHashes() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT rendered_hash FROM screens
		WHERE rendered_hash IS NOT NULL AND rendered_hash <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ScreenSettings returns the settings JSON of every screen whose plugin is of
// the given type. Used to discover which uploaded assets are still referenced.
func (s *Store) ScreenSettings(pluginType string) ([]string, error) {
	rows, err := s.db.Query(`SELECT s.settings_json FROM screens s
		JOIN plugins p ON p.id = s.plugin_id WHERE p.type = ?`, pluginType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var j string
		if err := rows.Scan(&j); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ClearScreenRendered forces a screen to re-render on next request.
func (s *Store) ClearScreenRendered(id int64) error {
	_, err := s.db.Exec(`UPDATE screens SET rendered_hash = NULL, rendered_at = NULL WHERE id = ?`, id)
	return err
}

// DeleteScreen removes a screen and its plugin instance.
func (s *Store) DeleteScreen(id int64) error {
	sc, err := s.GetScreen(id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM plugins WHERE id = ?`, sc.PluginID) // cascades to screen
	return err
}

// --- Playlists ---

// CreatePlaylist inserts a playlist.
func (s *Store) CreatePlaylist(name string) (*Playlist, error) {
	res, err := s.db.Exec(`INSERT INTO playlists (name, created_at) VALUES (?, unixepoch())`, name)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetPlaylist(id)
}

// GetPlaylist returns a playlist by ID.
func (s *Store) GetPlaylist(id int64) (*Playlist, error) {
	var p Playlist
	err := s.db.QueryRow(`SELECT id, name, created_at FROM playlists WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPlaylists returns all playlists.
func (s *Store) ListPlaylists() ([]*Playlist, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM playlists ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Playlist
	for rows.Next() {
		var p Playlist
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// DeletePlaylist removes a playlist and its items (via cascade).
func (s *Store) DeletePlaylist(id int64) error {
	_, err := s.db.Exec(`DELETE FROM playlists WHERE id = ?`, id)
	return err
}

// AddPlaylistItem appends a screen to a playlist at the next position.
func (s *Store) AddPlaylistItem(playlistID, screenID int64) error {
	_, err := s.db.Exec(`INSERT INTO playlist_items (playlist_id, screen_id, position, visible)
		VALUES (?, ?, COALESCE((SELECT MAX(position)+1 FROM playlist_items WHERE playlist_id = ?), 0), 1)`,
		playlistID, screenID, playlistID)
	return err
}

// RemovePlaylistItem deletes a playlist item by its ID.
func (s *Store) RemovePlaylistItem(itemID int64) error {
	_, err := s.db.Exec(`DELETE FROM playlist_items WHERE id = ?`, itemID)
	return err
}

// ListPlaylistItems returns visible items of a playlist ordered by position.
func (s *Store) ListPlaylistItems(playlistID int64) ([]*PlaylistItem, error) {
	rows, err := s.db.Query(`SELECT id, playlist_id, screen_id, position, visible
		FROM playlist_items WHERE playlist_id = ? ORDER BY position`, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*PlaylistItem
	for rows.Next() {
		var it PlaylistItem
		if err := rows.Scan(&it.ID, &it.PlaylistID, &it.ScreenID, &it.Position, &it.Visible); err != nil {
			return nil, err
		}
		out = append(out, &it)
	}
	return out, rows.Err()
}

// --- Settings ---

// GetSetting returns a setting value and whether it was present.
func (s *Store) GetSetting(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetSetting upserts a setting value.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
