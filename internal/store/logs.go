package store

import "time"

const logColumns = `id, device_id, log_id, message, created_at, received_at,
	wifi_status, wifi_signal, sleep_duration, refresh_rate, free_heap_size,
	max_alloc_size, source_path, source_line, wake_reason, firmware_version,
	battery_voltage, special_function`

// InsertLogs inserts a batch of device log entries in a single transaction.
func (s *Store) InsertLogs(logs []*DeviceLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO device_logs
		(device_id, log_id, message, created_at, received_at, wifi_status, wifi_signal,
		 sleep_duration, refresh_rate, free_heap_size, max_alloc_size, source_path,
		 source_line, wake_reason, firmware_version, battery_voltage, special_function)
		VALUES (?, ?, ?, ?, unixepoch(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, l := range logs {
		if _, err := stmt.Exec(l.DeviceID, l.LogID, l.Message, l.CreatedAt, l.WifiStatus,
			l.WifiSignal, l.SleepDuration, l.RefreshRate, l.FreeHeapSize, l.MaxAllocSize,
			l.SourcePath, l.SourceLine, l.WakeReason, l.FirmwareVersion, l.BatteryVoltage,
			l.SpecialFunction); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// PruneLogs deletes device log entries older than the given age and returns the
// number removed. A non-positive age is a no-op.
func (s *Store) PruneLogs(olderThan time.Duration) (int64, error) {
	if olderThan <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-olderThan).Unix()
	res, err := s.db.Exec(`DELETE FROM device_logs WHERE received_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ListLogs returns the most recent log entries for a device, newest first.
func (s *Store) ListLogs(deviceID int64, limit int) ([]*DeviceLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT `+logColumns+` FROM device_logs
		WHERE device_id = ? ORDER BY received_at DESC, id DESC LIMIT ?`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*DeviceLog
	for rows.Next() {
		var l DeviceLog
		if err := rows.Scan(&l.ID, &l.DeviceID, &l.LogID, &l.Message, &l.CreatedAt, &l.ReceivedAt,
			&l.WifiStatus, &l.WifiSignal, &l.SleepDuration, &l.RefreshRate, &l.FreeHeapSize,
			&l.MaxAllocSize, &l.SourcePath, &l.SourceLine, &l.WakeReason, &l.FirmwareVersion,
			&l.BatteryVoltage, &l.SpecialFunction); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}
