package store

import (
	"database/sql"
	"errors"
	"strings"
)

// --- Calendar accounts ---

const calendarAccountColumns = `id, provider, name, marker, config, refresh_interval, last_sync, last_error, created_at`

func scanCalendarAccount(row interface{ Scan(...any) error }) (*CalendarAccount, error) {
	var a CalendarAccount
	err := row.Scan(&a.ID, &a.Provider, &a.Name, &a.Marker, &a.ConfigJSON,
		&a.RefreshInterval, &a.LastSync, &a.LastError, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateCalendarAccount inserts a calendar account and returns it.
func (s *Store) CreateCalendarAccount(provider, name, marker, configJSON string, refreshInterval int) (*CalendarAccount, error) {
	if configJSON == "" {
		configJSON = "{}"
	}
	if refreshInterval <= 0 {
		refreshInterval = 12 * 60 * 60
	}
	res, err := s.db.Exec(`INSERT INTO calendar_accounts (provider, name, marker, config, refresh_interval, created_at)
		VALUES (?, ?, ?, ?, ?, unixepoch())`, provider, name, marker, configJSON, refreshInterval)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetCalendarAccount(id)
}

// GetCalendarAccount returns an account by ID.
func (s *Store) GetCalendarAccount(id int64) (*CalendarAccount, error) {
	return scanCalendarAccount(s.db.QueryRow(`SELECT `+calendarAccountColumns+` FROM calendar_accounts WHERE id = ?`, id))
}

// ListCalendarAccounts returns all accounts, oldest first (stable display order).
func (s *Store) ListCalendarAccounts() ([]*CalendarAccount, error) {
	rows, err := s.db.Query(`SELECT ` + calendarAccountColumns + ` FROM calendar_accounts ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CalendarAccount
	for rows.Next() {
		a, err := scanCalendarAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateCalendarAccount updates the editable fields and the provider config
// (used both for user edits and to persist refreshed OAuth tokens).
func (s *Store) UpdateCalendarAccount(id int64, name, marker, configJSON string, refreshInterval int) error {
	if configJSON == "" {
		configJSON = "{}"
	}
	if refreshInterval <= 0 {
		refreshInterval = 12 * 60 * 60
	}
	_, err := s.db.Exec(`UPDATE calendar_accounts SET name = ?, marker = ?, config = ?, refresh_interval = ? WHERE id = ?`,
		name, marker, configJSON, refreshInterval, id)
	return err
}

// SetCalendarAccountConfig persists only the provider config JSON, e.g. after
// an OAuth token refresh during sync.
func (s *Store) SetCalendarAccountConfig(id int64, configJSON string) error {
	if configJSON == "" {
		configJSON = "{}"
	}
	_, err := s.db.Exec(`UPDATE calendar_accounts SET config = ? WHERE id = ?`, configJSON, id)
	return err
}

// SetCalendarAccountSync records the outcome of a sync attempt. A non-empty
// errMsg is stored; an empty one clears any previous error.
func (s *Store) SetCalendarAccountSync(id int64, when int64, errMsg string) error {
	var e sql.NullString
	if errMsg != "" {
		e = sql.NullString{String: errMsg, Valid: true}
	}
	_, err := s.db.Exec(`UPDATE calendar_accounts SET last_sync = ?, last_error = ? WHERE id = ?`, when, e, id)
	return err
}

// DeleteCalendarAccount removes an account (cascading to its cached events).
func (s *Store) DeleteCalendarAccount(id int64) error {
	_, err := s.db.Exec(`DELETE FROM calendar_accounts WHERE id = ?`, id)
	return err
}

// --- Calendar events cache ---

// ReplaceCalendarEvents atomically replaces all cached events for an account
// with evs. Used after a successful sync.
func (s *Store) ReplaceCalendarEvents(accountID int64, evs []CalendarEvent, syncedAt int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.Exec(`DELETE FROM calendar_events WHERE account_id = ?`, accountID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO calendar_events
		(account_id, uid, title, start_at, end_at, all_day, location, status, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, e := range evs {
		if _, err = stmt.Exec(accountID, e.UID, e.Title, e.StartAt, e.EndAt, boolToInt(e.AllDay), e.Location, e.Status, syncedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListCalendarEvents returns cached events for the given account IDs whose start
// falls in [from, to), ordered by start. An empty accountIDs slice matches all
// accounts.
func (s *Store) ListCalendarEvents(accountIDs []int64, from, to int64) ([]CalendarEvent, error) {
	q := `SELECT id, account_id, uid, title, start_at, end_at, all_day, location, status, synced_at
		FROM calendar_events WHERE start_at >= ? AND start_at < ?`
	args := []any{from, to}
	if len(accountIDs) > 0 {
		q += ` AND account_id IN (` + placeholders(len(accountIDs)) + `)`
		for _, id := range accountIDs {
			args = append(args, id)
		}
	}
	q += ` ORDER BY start_at ASC, id ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarEvent
	for rows.Next() {
		var e CalendarEvent
		var allDay int
		if err := rows.Scan(&e.ID, &e.AccountID, &e.UID, &e.Title, &e.StartAt, &e.EndAt, &allDay, &e.Location, &e.Status, &e.SyncedAt); err != nil {
			return nil, err
		}
		e.AllDay = allDay != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// placeholders returns "?, ?, ..." with n placeholders for an IN clause.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
