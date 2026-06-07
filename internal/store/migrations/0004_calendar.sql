-- Calendar accounts (shared, configured once in admin) and a cache of fetched
-- events. The familycalendar plugin reads the cache; a background job fills it.

CREATE TABLE calendar_accounts (
    id               INTEGER PRIMARY KEY,
    provider         TEXT    NOT NULL,            -- 'google' (phase 1), 'caldav' (phase 2)
    name             TEXT    NOT NULL,            -- display name, e.g. "Mom"
    marker           TEXT    NOT NULL DEFAULT '', -- 1-2 char badge shown in the agenda
    config           TEXT    NOT NULL DEFAULT '{}', -- provider JSON (token, selected calendar ids, email)
    refresh_interval INTEGER NOT NULL DEFAULT 43200, -- per-account sync cadence, seconds (default 12h)
    last_sync        INTEGER,
    last_error       TEXT,
    created_at       INTEGER NOT NULL
);

CREATE TABLE calendar_events (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES calendar_accounts(id) ON DELETE CASCADE,
    uid        TEXT    NOT NULL,             -- iCalUID, used for cross-account dedup
    title      TEXT    NOT NULL,
    start_at   INTEGER NOT NULL,            -- unix seconds
    end_at     INTEGER NOT NULL,
    all_day    INTEGER NOT NULL DEFAULT 0,
    location   TEXT    NOT NULL DEFAULT '',
    status     TEXT    NOT NULL DEFAULT '',
    synced_at  INTEGER NOT NULL,
    UNIQUE(account_id, uid, start_at)       -- one row per recurrence instance
);

CREATE INDEX idx_calendar_events_time ON calendar_events(start_at);
CREATE INDEX idx_calendar_events_account ON calendar_events(account_id);
