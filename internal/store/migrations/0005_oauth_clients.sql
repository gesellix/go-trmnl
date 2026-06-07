-- OAuth clients for the family calendar's Google integration. Multiple clients
-- are supported (e.g. one per Google Cloud project); each Google calendar
-- account is bound to one client. The client secret is encrypted at rest when a
-- secret key is configured.
CREATE TABLE oauth_clients (
    id            INTEGER PRIMARY KEY,
    provider      TEXT    NOT NULL,            -- 'google'
    name          TEXT    NOT NULL,            -- display name, e.g. "Family GCP project"
    client_id     TEXT    NOT NULL,
    client_secret TEXT    NOT NULL,
    created_at    INTEGER NOT NULL
);
