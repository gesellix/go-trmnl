package store_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/store"
)

func openTest(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func newDevice(t *testing.T, st *store.Store, mac string) *store.Device {
	t.Helper()
	d, err := st.CreateDevice(&store.Device{MAC: mac, APIKey: "key-" + mac, FriendlyID: "F" + mac[len(mac)-2:]})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	return d
}

func TestDeviceCRUD(t *testing.T) {
	st := openTest(t)
	d := newDevice(t, st, "AA:BB:CC:DD:EE:01")

	// Defaults applied on insert.
	if d.Width != 800 || d.Height != 480 || d.RefreshRate != 900 {
		t.Errorf("defaults wrong: %dx%d @ %ds", d.Width, d.Height, d.RefreshRate)
	}

	byID, err := st.GetDeviceByID(d.ID)
	if err != nil || byID.MAC != d.MAC {
		t.Fatalf("GetDeviceByID: %+v err=%v", byID, err)
	}
	byMAC, err := st.GetDeviceByMAC(d.MAC)
	if err != nil || byMAC.ID != d.ID {
		t.Fatalf("GetDeviceByMAC: %+v err=%v", byMAC, err)
	}

	if _, err := st.GetDeviceByMAC("00:00:00:00:00:00"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing device err = %v, want ErrNotFound", err)
	}

	// Duplicate MAC is rejected by the UNIQUE constraint.
	if _, err := st.CreateDevice(&store.Device{MAC: d.MAC, APIKey: "x", FriendlyID: "X"}); err == nil {
		t.Errorf("expected duplicate MAC to fail")
	}
}

func TestUpdateTelemetryCoalesces(t *testing.T) {
	st := openTest(t)
	d := newDevice(t, st, "AA:BB:CC:DD:EE:02")

	// First update sets several fields.
	err := st.UpdateTelemetry(d.ID, store.Telemetry{
		FWVersion:      sql.NullString{String: "1.5.2", Valid: true},
		BatteryVoltage: sql.NullFloat64{Float64: 3.9, Valid: true},
		RSSI:           sql.NullInt64{Int64: -60, Valid: true},
		Width:          sql.NullInt64{Int64: 800, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Second update leaves FWVersion nil; it must be preserved (COALESCE).
	if err := st.UpdateTelemetry(d.ID, store.Telemetry{
		BatteryVoltage: sql.NullFloat64{Float64: 4.1, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	got, _ := st.GetDeviceByID(d.ID)
	if got.FWVersion.String != "1.5.2" {
		t.Errorf("fw_version not preserved: %q", got.FWVersion.String)
	}
	if got.BatteryVoltage.Float64 != 4.1 {
		t.Errorf("battery not updated: %v", got.BatteryVoltage.Float64)
	}
	if !got.LastSeenAt.Valid || got.LastSeenAt.Int64 == 0 {
		t.Errorf("last_seen_at not stamped")
	}
}

func TestFirmwareAndSpecialFunction(t *testing.T) {
	st := openTest(t)
	d := newDevice(t, st, "AA:BB:CC:DD:EE:03")

	// Default special function is "none".
	if d.SpecialFunction.String != "none" {
		t.Errorf("default special_function = %q, want none", d.SpecialFunction.String)
	}

	if err := st.QueueFirmwareUpdate(d.ID, "http://x/fw.bin"); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetDeviceByID(d.ID)
	if !got.FirmwareUpdate || got.FirmwareURL.String != "http://x/fw.bin" {
		t.Fatalf("firmware not queued: %+v", got)
	}
	if err := st.ClearFirmwareUpdate(d.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = st.GetDeviceByID(d.ID); got.FirmwareUpdate {
		t.Errorf("firmware update not cleared")
	}

	_ = st.QueueResetFirmware(d.ID)
	if got, _ = st.GetDeviceByID(d.ID); !got.ResetFirmware {
		t.Errorf("reset not queued")
	}
	_ = st.ClearResetFirmware(d.ID)
	if got, _ = st.GetDeviceByID(d.ID); got.ResetFirmware {
		t.Errorf("reset not cleared")
	}

	_ = st.SetSpecialFunction(d.ID, "sleep")
	if got, _ = st.GetDeviceByID(d.ID); got.SpecialFunction.String != "sleep" {
		t.Errorf("special_function = %q, want sleep", got.SpecialFunction.String)
	}
	// Empty falls back to "none".
	_ = st.SetSpecialFunction(d.ID, "")
	if got, _ = st.GetDeviceByID(d.ID); got.SpecialFunction.String != "none" {
		t.Errorf("special_function = %q, want none", got.SpecialFunction.String)
	}
}

func TestUpdateDeviceSettingsAndCursor(t *testing.T) {
	st := openTest(t)
	d := newDevice(t, st, "AA:BB:CC:DD:EE:04")
	pl, _ := st.CreatePlaylist("P")

	if err := st.UpdateDeviceSettings(d.ID, "Kitchen", 1200, sql.NullInt64{Int64: pl.ID, Valid: true}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetDeviceByID(d.ID)
	if got.Name.String != "Kitchen" || got.RefreshRate != 1200 || got.PlaylistID.Int64 != pl.ID {
		t.Fatalf("settings not saved: %+v", got)
	}

	_ = st.SetPlaylistCursor(d.ID, 3)
	if got, _ = st.GetDeviceByID(d.ID); got.PlaylistCursor != 3 {
		t.Errorf("cursor = %d, want 3", got.PlaylistCursor)
	}
}

func TestLogsInsertAndList(t *testing.T) {
	st := openTest(t)
	d := newDevice(t, st, "AA:BB:CC:DD:EE:05")

	if err := st.InsertLogs(nil); err != nil {
		t.Errorf("empty insert should be a no-op: %v", err)
	}

	logs := []*store.DeviceLog{
		{DeviceID: d.ID, Message: sql.NullString{String: "first", Valid: true}},
		{DeviceID: d.ID, Message: sql.NullString{String: "second", Valid: true}},
	}
	if err := st.InsertLogs(logs); err != nil {
		t.Fatal(err)
	}

	got, err := st.ListLogs(d.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d logs, want 2", len(got))
	}

	// Limit is honored.
	if one, _ := st.ListLogs(d.ID, 1); len(one) != 1 {
		t.Errorf("limit not honored: got %d", len(one))
	}
}

func TestScreensCRUD(t *testing.T) {
	st := openTest(t)
	pg, err := st.CreatePlugin("clock", "Clock")
	if err != nil {
		t.Fatal(err)
	}
	// Empty settings default to "{}".
	sc, err := st.CreateScreen(pg.ID, "Clock", "")
	if err != nil {
		t.Fatal(err)
	}
	if sc.SettingsJSON != "{}" {
		t.Errorf("default settings = %q, want {}", sc.SettingsJSON)
	}

	_ = st.SetScreenRendered(sc.ID, "deadbeef")
	if got, _ := st.GetScreen(sc.ID); !got.RenderedHash.Valid || got.RenderedHash.String != "deadbeef" {
		t.Errorf("rendered hash not set")
	}

	// Updating settings clears the cached render.
	if err := st.UpdateScreenSettings(sc.ID, "Clock 2", `{"use_24h":true}`); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetScreen(sc.ID)
	if got.Name != "Clock 2" || got.RenderedHash.Valid {
		t.Errorf("update should rename and clear render: %+v", got)
	}

	_ = st.SetScreenRendered(sc.ID, "cafe")
	_ = st.ClearScreenRendered(sc.ID)
	if got, _ = st.GetScreen(sc.ID); got.RenderedHash.Valid {
		t.Errorf("ClearScreenRendered did not clear")
	}

	// DeleteScreen removes the plugin instance (which cascades the screen).
	if err := st.DeleteScreen(sc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetScreen(sc.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("screen still present after delete: %v", err)
	}
	if _, err := st.GetPlugin(pg.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("plugin still present after screen delete: %v", err)
	}
}

func TestPlaylistsAndItems(t *testing.T) {
	st := openTest(t)
	pl, _ := st.CreatePlaylist("Default")
	pg, _ := st.CreatePlugin("clock", "c")
	s1, _ := st.CreateScreen(pg.ID, "a", "{}")
	s2, _ := st.CreateScreen(pg.ID, "b", "{}")

	_ = st.AddPlaylistItem(pl.ID, s1.ID)
	_ = st.AddPlaylistItem(pl.ID, s2.ID)

	items, _ := st.ListPlaylistItems(pl.ID)
	if len(items) != 2 || items[0].Position != 0 || items[1].Position != 1 {
		t.Fatalf("positions not assigned sequentially: %+v", items)
	}

	_ = st.RemovePlaylistItem(items[0].ID)
	if items, _ = st.ListPlaylistItems(pl.ID); len(items) != 1 {
		t.Errorf("item not removed: %d remain", len(items))
	}

	if pls, _ := st.ListPlaylists(); len(pls) != 1 {
		t.Errorf("ListPlaylists = %d, want 1", len(pls))
	}

	// Deleting the playlist cascades its items.
	_ = st.DeletePlaylist(pl.ID)
	if items, _ = st.ListPlaylistItems(pl.ID); len(items) != 0 {
		t.Errorf("items not cascaded on playlist delete: %d", len(items))
	}
}

func TestSettings(t *testing.T) {
	st := openTest(t)
	if _, ok, _ := st.GetSetting("missing"); ok {
		t.Errorf("absent setting reported present")
	}
	_ = st.SetSetting("dither_mode", "threshold")
	if v, ok, _ := st.GetSetting("dither_mode"); !ok || v != "threshold" {
		t.Errorf("setting = %q ok=%v", v, ok)
	}
	// Upsert overwrites.
	_ = st.SetSetting("dither_mode", "floyd_steinberg")
	if v, _, _ := st.GetSetting("dither_mode"); v != "floyd_steinberg" {
		t.Errorf("upsert failed: %q", v)
	}
}

func TestDeleteDeviceCascadesLogs(t *testing.T) {
	st := openTest(t)
	d := newDevice(t, st, "AA:BB:CC:DD:EE:06")
	_ = st.InsertLogs([]*store.DeviceLog{{DeviceID: d.ID, Message: sql.NullString{String: "x", Valid: true}}})

	if err := st.DeleteDevice(d.ID); err != nil {
		t.Fatal(err)
	}
	if logs, _ := st.ListLogs(d.ID, 10); len(logs) != 0 {
		t.Errorf("logs not cascaded on device delete: %d", len(logs))
	}
}

func TestScreenSettingsByPluginType(t *testing.T) {
	st := openTest(t)

	pgImg, _ := st.CreatePlugin("staticimage", "img")
	st.CreateScreen(pgImg.ID, "img", `{"file":"a.png"}`)
	pgClock, _ := st.CreatePlugin("clock", "c")
	st.CreateScreen(pgClock.ID, "c", `{"use_24h":true}`)

	got, err := st.ScreenSettings("staticimage")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != `{"file":"a.png"}` {
		t.Errorf("staticimage settings = %v, want one image screen", got)
	}
	if other, _ := st.ScreenSettings("weather"); len(other) != 0 {
		t.Errorf("weather settings = %v, want none", other)
	}
}

func TestPruneLogs(t *testing.T) {
	st := openTest(t)
	d := newDevice(t, st, "AA:BB:CC:DD:EE:08")

	// Insert two rows with controlled received_at: one old, one fresh.
	oldTS := time.Now().Add(-40 * 24 * time.Hour).Unix()
	freshTS := time.Now().Unix()
	for _, ts := range []int64{oldTS, freshTS} {
		if _, err := st.DB().Exec(
			`INSERT INTO device_logs (device_id, message, received_at) VALUES (?, ?, ?)`,
			d.ID, "m", ts); err != nil {
			t.Fatal(err)
		}
	}

	// Retain 32 days: the 40-day-old row should go, the fresh one stays.
	n, err := st.PruneLogs(32 * 24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("pruned %d rows, want 1", n)
	}
	remaining, _ := st.ListLogs(d.ID, 10)
	if len(remaining) != 1 {
		t.Errorf("remaining logs = %d, want 1", len(remaining))
	}

	// Zero/negative retention is a no-op.
	if n, _ := st.PruneLogs(0); n != 0 {
		t.Errorf("PruneLogs(0) removed %d, want 0", n)
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	st1, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = newDevice(t, st1, "AA:BB:CC:DD:EE:07")
	_ = st1.Close()

	// Reopening re-runs migrate; it must not error or wipe data.
	st2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = st2.Close() })
	if _, err := st2.GetDeviceByMAC("AA:BB:CC:DD:EE:07"); err != nil {
		t.Errorf("data lost after reopen: %v", err)
	}
}
