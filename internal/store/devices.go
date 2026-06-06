package store

import (
	"database/sql"
	"errors"
)

const deviceColumns = `id, mac, api_key, friendly_id, name, model, fw_version,
	width, height, refresh_rate, playlist_id, playlist_cursor,
	battery_voltage, battery_charging, rssi, wifi_status, last_seen_at, created_at`

func scanDevice(row interface{ Scan(...any) error }) (*Device, error) {
	var d Device
	err := row.Scan(&d.ID, &d.MAC, &d.APIKey, &d.FriendlyID, &d.Name, &d.Model, &d.FWVersion,
		&d.Width, &d.Height, &d.RefreshRate, &d.PlaylistID, &d.PlaylistCursor,
		&d.BatteryVoltage, &d.BatteryCharging, &d.RSSI, &d.WifiStatus, &d.LastSeenAt, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// CreateDevice inserts a new device and returns it with its assigned ID.
func (s *Store) CreateDevice(d *Device) (*Device, error) {
	res, err := s.db.Exec(`INSERT INTO devices
		(mac, api_key, friendly_id, name, model, fw_version, width, height, refresh_rate, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())`,
		d.MAC, d.APIKey, d.FriendlyID, d.Name, d.Model, d.FWVersion,
		nz(d.Width, 800), nz(d.Height, 480), nz(d.RefreshRate, 900))
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetDeviceByID(id)
}

// GetDeviceByID returns the device with the given primary key.
func (s *Store) GetDeviceByID(id int64) (*Device, error) {
	return scanDevice(s.db.QueryRow(`SELECT `+deviceColumns+` FROM devices WHERE id = ?`, id))
}

// GetDeviceByMAC returns the device with the given MAC address.
func (s *Store) GetDeviceByMAC(mac string) (*Device, error) {
	return scanDevice(s.db.QueryRow(`SELECT `+deviceColumns+` FROM devices WHERE mac = ?`, mac))
}

// ListDevices returns all devices ordered by most recently seen.
func (s *Store) ListDevices() ([]*Device, error) {
	rows, err := s.db.Query(`SELECT ` + deviceColumns + ` FROM devices ORDER BY last_seen_at DESC NULLS LAST, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Telemetry holds the subset of device fields updated on each display poll.
type Telemetry struct {
	FWVersion       sql.NullString
	Model           sql.NullString
	Width           sql.NullInt64
	Height          sql.NullInt64
	BatteryVoltage  sql.NullFloat64
	BatteryCharging sql.NullBool
	RSSI            sql.NullInt64
	WifiStatus      sql.NullString
	RefreshRate     sql.NullInt64
}

// UpdateTelemetry persists the latest telemetry for a device and stamps
// last_seen_at. Only non-null fields overwrite existing values.
func (s *Store) UpdateTelemetry(deviceID int64, t Telemetry) error {
	_, err := s.db.Exec(`UPDATE devices SET
		fw_version       = COALESCE(?, fw_version),
		model            = COALESCE(?, model),
		width            = COALESCE(?, width),
		height           = COALESCE(?, height),
		battery_voltage  = COALESCE(?, battery_voltage),
		battery_charging = COALESCE(?, battery_charging),
		rssi             = COALESCE(?, rssi),
		wifi_status      = COALESCE(?, wifi_status),
		refresh_rate     = COALESCE(?, refresh_rate),
		last_seen_at     = unixepoch()
		WHERE id = ?`,
		t.FWVersion, t.Model, t.Width, t.Height, t.BatteryVoltage, t.BatteryCharging,
		t.RSSI, t.WifiStatus, t.RefreshRate, deviceID)
	return err
}

// UpdateDeviceSettings updates the user-editable fields of a device.
func (s *Store) UpdateDeviceSettings(id int64, name string, refreshRate int, playlistID sql.NullInt64) error {
	_, err := s.db.Exec(`UPDATE devices SET name = ?, refresh_rate = ?, playlist_id = ? WHERE id = ?`,
		name, refreshRate, playlistID, id)
	return err
}

// SetPlaylistCursor stores the round-robin cursor for a device.
func (s *Store) SetPlaylistCursor(id int64, cursor int) error {
	_, err := s.db.Exec(`UPDATE devices SET playlist_cursor = ? WHERE id = ?`, cursor, id)
	return err
}

// DeleteDevice removes a device and its logs (via cascade).
func (s *Store) DeleteDevice(id int64) error {
	_, err := s.db.Exec(`DELETE FROM devices WHERE id = ?`, id)
	return err
}

func nz(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
