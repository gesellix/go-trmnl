-- Per-device battery indicator in the rendered screen's footer.
ALTER TABLE devices ADD COLUMN show_battery INTEGER NOT NULL DEFAULT 0;

-- The rendered image is cached by content hash on the screen row, which
-- assumes every device sees the same picture. The battery indicator makes a
-- render device-specific, so devices that show it need their own cache entry.
CREATE TABLE IF NOT EXISTS device_screen_renders (
  device_id   INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  screen_id   INTEGER NOT NULL REFERENCES screens(id) ON DELETE CASCADE,
  rendered_hash TEXT NOT NULL,
  rendered_at INTEGER NOT NULL,
  -- What the indicator showed in that render, so a changed battery level
  -- invalidates the entry even while the plugin's own cache is still fresh.
  battery_state INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (device_id, screen_id)
);
