# Family Calendar Plugin

Type: `familycalendar`. Displays upcoming family events as an agenda grouped by days, merged from one or more **calendar accounts**. Additionally, weather information can be displayed in a sidebar. Events appearing in multiple accounts (e.g., a shared invitation accepted by both parents) are merged into a single entry showing the markers of all involved accounts.

[← All plugins](../PLUGINS.md)

## Organization

Instead of storing credentials per screen, **calendar accounts** are configured once under **Admin → Calendar** (each with a short marker, e.g., `M` for Mom). Calendar screens then simply select which accounts should be displayed. A background process synchronizes each account regularly into a local cache; the screens render from this cache, making them very fast.

Supported providers:

- **Google** (OAuth2) — see [Setting up Google](#setting-up-google).
- **Apple iCloud / CalDAV** (app-specific password) — see
  [Setting up Apple and CalDAV](#setting-up-apple-and-caldav).

You can mix different accounts (e.g., one Google account per parent and an iCloud account for a child).

## Layout

The plugin uses a two-column layout:
- **Left:** The event overview (agenda).
- **Right:** Weather information (current temperature, weather icon, and a forecast for tomorrow), if coordinates are configured.

A legend of the person markers and the time of the last update are displayed at the bottom.

## Settings

| Key          | Type    | Default  | Notes                                           |
|--------------|---------|----------|-------------------------------------------------|
| `accounts`   | array   | (all)    | Account IDs to include; empty/omitted means all |
| `days`       | number  | `14`     | How many days in advance to display             |
| `max_events` | number  | `30`     | Upper limit for the number of displayed events  |
| `label`      | string  | (empty)  | Optional title displayed at the top             |
| `use_24h`    | boolean | `false`  | 24-hour format (`15:04`) instead of `3:04 PM`   |
| `location`   | string  | (empty)  | Location name for weather (e.g., "London")      |
| `latitude`   | number  | `0`      | Latitude for weather data                       |
| `longitude`  | number  | `0`      | Longitude for weather data                      |
| `units`      | string  | `metric` | `metric` (°C, km/h) or `imperial` (°F, mph)     |

```json
{
  "label": "Family",
  "accounts": [1, 2],
  "days": 14,
  "use_24h": true,
  "location": "London",
  "latitude": 51.5074,
  "longitude": -0.1278,
  "units": "metric"
}
```

In the admin interface, you don't need to edit this JSON manually: The editor shows a checklist of your calendar accounts and fields for the options mentioned above. If no account is selected, all are included.

Cache notice: approx. 15 minutes (interval at which the device reloads the rendered image). The underlying calendar data is updated per account at its own interval (default 12h, adjustable under **Admin → Calendar**).

## Setting up Google

Reading private Google calendars requires OAuth2; there is no public-link
shortcut. You create one (or more) OAuth clients in Google Cloud and add them in
the admin UI; each Google account is then bound to a chosen client. Multiple
clients are supported (e.g. one per Google Cloud project).

### 1. Create an OAuth client

Google recently moved these settings into the **Google Auth Platform** section;
the classic **APIs & Services** paths are noted in parentheses where they differ.

1. Open the [Google Cloud Console](https://console.cloud.google.com/) and create
   (or pick) a project.
2. **APIs & Services → Library →** enable the **Google Calendar API**.
3. Configure the consent screen under **Google Auth Platform → Branding** and
   **→ Audience** (classic: **APIs & Services → OAuth consent screen**):
   - **Audience / User type: External** is fine for personal Gmail.
   - Fill in the app name and your contact email.
   - Under **Audience → Test users**, add each family member's Google address. A
     "Testing" app is limited to listed test users, which is exactly what a
     family server wants; you do not need to publish or get verified.
4. Add the read-only scope under **Google Auth Platform → Data access → Add or
   remove scopes** (classic: the **Scopes** step of the consent screen): search
   the Google Calendar API and select `.../auth/calendar.readonly`. (go-trmnl
   also requests this scope at sign-in, so test users are prompted for it
   regardless.)
5. Create the client under **Google Auth Platform → Clients → Create client**
   (classic: **APIs & Services → Credentials → Create credentials → OAuth client
   ID**):
   - Application type **Web application**.
   - Under **Authorized redirect URIs**, add exactly:

     ```
     http://<host>:8080/admin/oauth/google/callback
     ```

     where `<host>` is the host you will open the admin UI with. **Google
     rejects raw private IP addresses** (e.g. `http://192.168.1.10:8080/...`
     fails with "device_id and device_name are required for private IP"), so
     use a DNS or mDNS hostname, e.g.
     `http://trmnl.local:8080/admin/oauth/google/callback`. It must match
     character-for-character, including the scheme and port.
6. Copy the generated **Client ID** and **Client secret**.

### 2. Add the OAuth client in go-trmnl

In **Admin → Calendar → Google OAuth clients**, add the client: a name, the
**Client ID** and the **Client secret** from step 1. The secret is stored in
the database, encrypted at rest when a key is configured (see
[`-secret-key`](../GETTING-STARTED.md)). You can add several clients; there are
no env/CLI variables for them.

> Note: the OAuth redirect URI is derived from the host you use to reach the
> admin UI, **not** from `-base-url`. So open the admin UI at
> `http://<host>:8080/admin/calendar` (the same hostname you registered) when
> adding Google accounts, and the redirect will match. This means `-base-url`
> can stay your LAN IP for the device's image fetches, while the OAuth flow uses
> the hostname Google requires. The hostname only needs to resolve in the
> browser you run the consent flow from.

### 3. Add accounts

1. **Admin → Calendar → Add Google account**, choosing the OAuth client to use.
   You are redirected to Google's consent screen; approve read-only calendar
   access.
2. Back in the admin UI, set a short **marker** (1–2 characters shown on the
   agenda), pick which of that account's **calendars** to include, and set the
   **refresh interval** (hours; default 12).
3. Repeat for each family member. Several accounts can share one client (add
   each as a Test user in Google Cloud), or use a different client per account.

### 4. Add a screen

**Screens → create (Family Calendar) → edit settings → Render preview.** Use the
`accounts` setting to choose which accounts this screen shows (omit it to show
all).

## Setting up Apple and CalDAV

Apple iCloud (and generic CalDAV servers) use HTTP basic auth. For iCloud you
must use an **app-specific password**, not your normal Apple ID password, and
the account needs two-factor authentication enabled.

1. At [appleid.apple.com](https://appleid.apple.com) → **Sign-In and Security →
   App-Specific Passwords**, generate a password for go-trmnl.
2. **Admin → Calendar →** under *Apple iCloud / CalDAV*, fill in:
   - **Apple ID / username** (your iCloud email),
   - **App-specific password** (the one you just generated),
   - **Server** — leave as `https://caldav.icloud.com` for iCloud, or point it
     at another CalDAV server,
   - a **marker** and **refresh interval**.
3. Submit. go-trmnl discovers your calendars and runs an initial sync (any auth
   error shows on the account row). Open the account to pick which calendars to
   include and adjust name/marker/refresh. With none selected, CalDAV defaults
   to the **first discovered calendar** (CalDAV has no reliable "primary", so
   pick the ones you want explicitly).

No global configuration is needed for CalDAV. App-specific passwords are stored
the same way as Google tokens (encrypted when `TRMNL_SECRET_KEY` is set).

## Testing

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

- **Google** expands recurring events server-side, so weekly meetings show as
  individual instances. Cancelled instances are dropped.
- **CalDAV/iCloud** events are fetched with an explicit property request
  (iCloud ignores `allprop`/`allcomp` and returns empty data for server-side
  `expand`). Recurring CalDAV events are expanded **client-side** from their
  `RRULE` (with `EXDATE`/`RDATE`) over the sync window, so a series that started
  in the past still shows its upcoming instances.
- Deduplication keys on the iCalUID plus start time, with a title+time fallback
  for events without a UID. Recurring instances stay distinct.
- All-day events are labeled "all day" and grouped on their date.
- Credentials (OAuth tokens, CalDAV passwords, OAuth client secrets) are stored
  in the local SQLite database and **encrypted at rest by default** (AES-256-GCM,
  key derived with scrypt). With no `-secret-key` / `TRMNL_SECRET_KEY` set, a key
  is generated on first start and saved to `<data-dir>/secret.key` (logged once);
  back it up, since losing it means re-entering the encrypted credentials. Pass
  `-secret-key` to supply your own, or `-no-encryption` to store plaintext.
  Existing plaintext entries are encrypted automatically on the next start.
