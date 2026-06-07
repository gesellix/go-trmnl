# Family Calendar plugin

Type: `familycalendar`. Shows the family's upcoming events as an agenda grouped
by day, merged from one or more **calendar accounts**. Events that appear on
several accounts (a shared invite both parents accepted) are deduplicated into a
single entry that shows every account's marker.

[← All plugins](../PLUGINS.md)

## How it is organized

Unlike other plugins, the calendar's credentials are **not** stored per screen.
You configure **calendar accounts** once under **Admin → Calendar** (each with a
short marker badge, e.g. `M` for Mom). Calendar screens then just pick which
accounts to show and how. A background job syncs each account into a local
cache on its own schedule; screens render from that cache, so they stay fast.

Currently supported account providers:

- **Google** (OAuth2) — see [Setting up Google](#setting-up-google) below.
- **Apple / CalDAV** — planned; the provider layer is already in place.

## Settings

| Key          | Type    | Default | Notes                                              |
|--------------|---------|---------|----------------------------------------------------|
| `accounts`   | array   | (all)   | Account IDs to include; empty/omitted means all    |
| `days`       | number  | `14`    | How many days ahead to show                        |
| `max_events` | number  | `30`    | Cap on the number of events displayed              |
| `label`      | string  | (empty) | Optional title shown at the top                    |
| `use_24h`    | boolean | `false` | 24-hour times (`15:04`) instead of `3:04 PM`       |

```json
{ "label": "Family", "accounts": [1, 2], "days": 14, "use_24h": true }
```

Cache TTL hint: ~15 minutes (how often a device re-fetches the rendered screen).
The underlying calendar data is refreshed per account on its own interval
(default 12h, set per account in **Admin → Calendar**).

## Setting up Google

Reading private Google calendars requires OAuth2; there is no public-link
shortcut. You create one OAuth client in Google Cloud and reuse it for every
family member's Google account.

### 1. Create an OAuth client

1. Open the [Google Cloud Console](https://console.cloud.google.com/) and create
   (or pick) a project.
2. **APIs & Services → Library →** enable the **Google Calendar API**.
3. **APIs & Services → OAuth consent screen:**
   - User type **External** (personal Gmail accounts) is fine.
   - Fill in the app name and your contact email.
   - Add the scope `.../auth/calendar.readonly` (read-only calendar access).
   - Add each family member's Google address under **Test users** (an app in
     "Testing" mode is limited to listed test users, which is exactly what a
     family server wants; you do not need to publish or get verified).
4. **APIs & Services → Credentials → Create credentials → OAuth client ID:**
   - Application type **Web application**.
   - Under **Authorized redirect URIs**, add exactly:

     ```
     <base-url>/admin/oauth/google/callback
     ```

     where `<base-url>` is your server's public base URL (the `-base-url` /
     `TRMNL_BASE_URL` value), e.g.
     `http://192.168.1.10:8080/admin/oauth/google/callback`. It must match
     character-for-character, including the scheme and port.
5. Copy the generated **Client ID** and **Client secret**.

### 2. Configure go-trmnl

Provide the credentials via environment variables (or the equivalent flags):

```sh
export TRMNL_GOOGLE_CLIENT_ID="…apps.googleusercontent.com"
export TRMNL_GOOGLE_CLIENT_SECRET="…"
# Also ensure the base URL matches the redirect URI host you registered:
export TRMNL_BASE_URL="http://192.168.1.10:8080"
```

Flags: `-google-client-id`, `-google-client-secret`. When unset, the calendar
page shows that Google integration is disabled.

> Note: the registered redirect URI must be reachable in your browser during the
> consent flow. If your `base-url` is a LAN IP, run the consent flow from a
> device on that LAN. (Google permits `http://` only for loopback; for a LAN IP
> you may need to front the server with HTTPS or use a loopback redirect during
> setup. A localhost base URL works for the OAuth dance but a physical TRMNL on
> the LAN cannot fetch images from it, so this is mainly a first-time-setup
> consideration.)

### 3. Add accounts

1. **Admin → Calendar → Add Google account.** You are redirected to Google's
   consent screen; approve read-only calendar access.
2. Back in the admin UI, set a short **marker** (1–2 characters shown on the
   agenda), pick which of that account's **calendars** to include, and set the
   **refresh interval** (hours; default 12).
3. Repeat **Add Google account** for each family member. The same OAuth client
   is reused; each consent yields a separate stored account.

### 4. Add a screen

**Screens → create (Family Calendar) → edit settings → Render preview.** Use the
`accounts` setting to choose which accounts this screen shows (omit it to show
all).

## How to test

The plugin reads from the local event cache, so pass a database path to the
render CLI after you have configured and synced at least one account:

```sh
go run ./cmd/trmnl-render -plugin familycalendar \
  -db ./data/trmnl.db \
  -settings '{"label":"Family","days":14,"use_24h":true}' -out cal
```

Without `-db` (or before any sync) the screen renders an empty "No upcoming
events" state. Rendering is separated from data fetching, so the unit tests in
`internal/plugins/familycalendar/` and `internal/calendar/` cover merging,
deduplication and drawing without any network access.

## Notes

- Recurring events are expanded server-side by Google (so weekly meetings show
  as individual instances). Cancelled instances are dropped.
- Deduplication keys on the iCalUID plus start time, with a title+time fallback
  for events without a UID. Recurring instances stay distinct.
- All-day events are labeled "all day" and grouped on their date.
- OAuth tokens are stored in the local SQLite database. Set `TRMNL_SECRET_KEY`
  to encrypt them at rest (AES-256-GCM); without it they are stored as
  plaintext. Tokens written while a key is set cannot be read after the key is
  removed or changed, so back up the key. Existing plaintext tokens keep working
  and are upgraded to encrypted form the next time their account is saved or its
  token is refreshed.
