package store

import (
	"database/sql"
	"errors"
)

const oauthClientColumns = `id, provider, name, client_id, client_secret, created_at`

func scanOAuthClient(row interface{ Scan(...any) error }) (*OAuthClient, error) {
	var c OAuthClient
	err := row.Scan(&c.ID, &c.Provider, &c.Name, &c.ClientID, &c.ClientSecret, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateOAuthClient inserts an OAuth client and returns it.
func (s *Store) CreateOAuthClient(provider, name, clientID, clientSecret string) (*OAuthClient, error) {
	res, err := s.db.Exec(`INSERT INTO oauth_clients (provider, name, client_id, client_secret, created_at)
		VALUES (?, ?, ?, ?, unixepoch())`, provider, name, clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetOAuthClient(id)
}

// GetOAuthClient returns a client by ID.
func (s *Store) GetOAuthClient(id int64) (*OAuthClient, error) {
	return scanOAuthClient(s.db.QueryRow(`SELECT `+oauthClientColumns+` FROM oauth_clients WHERE id = ?`, id))
}

// ListOAuthClients returns all clients, oldest first.
func (s *Store) ListOAuthClients() ([]*OAuthClient, error) {
	rows, err := s.db.Query(`SELECT ` + oauthClientColumns + ` FROM oauth_clients ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*OAuthClient
	for rows.Next() {
		c, err := scanOAuthClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetOAuthClientSecret updates only the stored client secret (used to
// re-encrypt it to a new format).
func (s *Store) SetOAuthClientSecret(id int64, clientSecret string) error {
	_, err := s.db.Exec(`UPDATE oauth_clients SET client_secret = ? WHERE id = ?`, clientSecret, id)
	return err
}

// DeleteOAuthClient removes a client. Accounts referencing it keep their stored
// tokens but can no longer be refreshed until re-authorized against a client.
func (s *Store) DeleteOAuthClient(id int64) error {
	_, err := s.db.Exec(`DELETE FROM oauth_clients WHERE id = ?`, id)
	return err
}
