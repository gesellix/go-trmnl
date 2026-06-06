-- Initial schema for the go-trmnl BYOS server.
-- All timestamps are stored as Unix seconds (INTEGER).

CREATE TABLE playlists (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE plugins (
    id         INTEGER PRIMARY KEY,
    type       TEXT    NOT NULL,          -- registry key: "clock", "staticimage", ...
    name       TEXT    NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE screens (
    id            INTEGER PRIMARY KEY,
    plugin_id     INTEGER NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    name          TEXT    NOT NULL,
    settings_json TEXT    NOT NULL DEFAULT '{}',
    rendered_hash TEXT,                   -- content hash of the current rendered image
    rendered_at   INTEGER,
    created_at    INTEGER NOT NULL
);

CREATE TABLE playlist_items (
    id          INTEGER PRIMARY KEY,
    playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    screen_id   INTEGER NOT NULL REFERENCES screens(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    visible     INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_playlist_items_playlist ON playlist_items(playlist_id, position);

CREATE TABLE devices (
    id               INTEGER PRIMARY KEY,
    mac              TEXT    NOT NULL UNIQUE,   -- the "ID" header
    api_key          TEXT    NOT NULL UNIQUE,   -- the "Access-Token" header
    friendly_id      TEXT    NOT NULL UNIQUE,
    name             TEXT,
    model            TEXT,
    fw_version       TEXT,
    width            INTEGER NOT NULL DEFAULT 800,
    height           INTEGER NOT NULL DEFAULT 480,
    refresh_rate     INTEGER NOT NULL DEFAULT 900,
    playlist_id      INTEGER REFERENCES playlists(id) ON DELETE SET NULL,
    playlist_cursor  INTEGER NOT NULL DEFAULT 0,
    -- last-seen telemetry
    battery_voltage  REAL,
    battery_charging INTEGER,
    rssi             INTEGER,
    wifi_status      TEXT,
    last_seen_at     INTEGER,
    created_at       INTEGER NOT NULL
);

CREATE TABLE device_logs (
    id               INTEGER PRIMARY KEY,
    device_id        INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    log_id           INTEGER,               -- "id" from the device payload
    message          TEXT,
    created_at       INTEGER,               -- unix from the device payload
    received_at      INTEGER NOT NULL,
    wifi_status      TEXT,
    wifi_signal      INTEGER,
    sleep_duration   INTEGER,
    refresh_rate     INTEGER,
    free_heap_size   INTEGER,
    max_alloc_size   INTEGER,
    source_path      TEXT,
    source_line      INTEGER,
    wake_reason      TEXT,
    firmware_version TEXT,
    battery_voltage  REAL,
    special_function TEXT
);

CREATE INDEX idx_device_logs_device ON device_logs(device_id, received_at DESC);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
