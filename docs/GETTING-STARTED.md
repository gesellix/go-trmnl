# Getting started with go-trmnl

This guide takes you from nothing to a TRMNL device showing screens served by
your own `go-trmnl` instance. It is self-contained; you should not need to
leave this repo to get running. Where something is on the device/firmware side
(and therefore outside this server's control), it links to the official TRMNL
documentation.

Related docs in this repo:

- [Device API reference](API.md) — the exact endpoints, headers and responses.
- [Plugins reference](PLUGINS.md) — built-in screens and their settings.
- [Design & milestones](PLAN.md) — how the server is built.

Official TRMNL references:

- [BYOS overview](https://docs.trmnl.com/go/diy/byos)
- [DIY introduction](https://docs.trmnl.com/go/diy/introduction)
- [Firmware](https://github.com/usetrmnl/firmware)

---

## What you'll need

- A **TRMNL device** (or compatible DIY hardware) running firmware that
  supports a custom server URL. See the official
  [BYOS overview](https://docs.trmnl.com/go/diy/byos) and
  [firmware repo](https://github.com/usetrmnl/firmware) for device/firmware
  setup and which versions support BYOS.
- A **host on the same LAN** as the device to run `go-trmnl` (a laptop, a NAS,
  a Raspberry Pi, etc.). The device must be able to reach this host directly.
- Either **Go 1.26+** or **Docker** on that host.

> The device fetches its screen image over plain HTTP from a URL this server
> hands out, so the server must be reachable from the device on your network.
> A loopback address like `127.0.0.1` will **not** work — use the host's LAN IP.

---

## 1. Run the server

First find your host's LAN IP (you'll pass it as the public base URL):

```sh
# macOS
ipconfig getifaddr en0
# Linux
hostname -I | awk '{print $1}'
```

### Option A — Go

```sh
git clone https://github.com/gesellix/go-trmnl
cd go-trmnl
go run ./cmd/trmnld -base-url http://<your-lan-ip>:8080
```

### Option B — Docker

```sh
docker build -t go-trmnl .
docker run -p 8080:8080 -v trmnl-data:/data \
  -e TRMNL_BASE_URL=http://<your-lan-ip>:8080 \
  -e TRMNL_ADMIN_PASSWORD=changeme \
  go-trmnl
```

On a fresh start the server creates its database and seeds an **Example**
playlist with a clock and a weather screen, so there is something to show right
away.

If you pass a loopback `-base-url`, the server logs a warning at startup —
that's your cue to use the LAN IP instead.

---

## 2. Open the admin UI

Browse to **`http://<your-lan-ip>:8080/admin`**.

If you set `TRMNL_ADMIN_PASSWORD` (recommended), you'll be prompted for the
username (`admin` by default) and that password. Without a password set, the
admin UI is open — fine for a trusted LAN, not for anything exposed.

You'll see the dashboard with counts for devices, screens and playlists.

---

## 3. Point your TRMNL device at the server

You tell the device to use `go-trmnl` through its Wi-Fi captive portal, under
**Advanced → Custom Server**. The steps below summarize the official guides;
refer to them for device-specific details and screenshots:

- [Enable Wi-Fi pairing mode](https://help.trmnl.com/en/articles/11511577-enable-wifi-pairing-mode)
- [Connect your device to a (Terminus) BYOS server](https://help.trmnl.com/en/articles/12263392-connect-your-device-to-terminus-byos)

**a. Put the device into Wi-Fi pairing mode.**

- *TRMNL OG (7.5")*: hold the **boot button** on the back for **6–8 seconds**,
  then release. If it doesn't enter pairing mode, try holding **15–20 seconds**.
- *TRMNL X (10.3")*: hold **both ends of the touchbar** until the screen blinks,
  then **hold the middle** of the touchbar to confirm entering pairing mode.

**b. Join the device's Wi-Fi.** From your phone or laptop, connect to the
Wi-Fi network named **`TRMNL`**. A captive-portal page opens automatically.

**c. Set the custom server.** In the portal, go to **Advanced → Custom Server →
Yes**, then enter your `go-trmnl` base URL in the format
`http://<your-lan-ip>:8080` — **no trailing slash** (e.g.
`http://192.168.1.10:8080`). The firmware appends `/api/...` itself, so enter
just the base URL.

**d. Connect to your Wi-Fi.** Tap **Back to Wi-Fi**, choose your network's SSID,
enter its password, and **Connect**. The device saves the credentials, reboots
onto your network, and calls your server. (Wi-Fi credentials are stored on the
device only.)

> **Verify the server first (optional but recommended).** Before involving
> hardware, simulate a device from any machine on the LAN:
>
> ```sh
> curl -H "ID: AA:BB:CC:DD:EE:FF" http://<your-lan-ip>:8080/api/setup
> ```
>
> A JSON response containing `api_key` and `friendly_id` means it's working.
> See the [Device API reference](API.md) for all endpoints.

---

## 4. Register your device

Before the device can connect, you must manually add its MAC address to the
server to build trust:

1. In the admin UI, go to **Devices**.
2. Enter the device's MAC address (e.g. `AA:BB:CC:DD:EE:FF`) into the **Add by MAC address** form and click **Add Device**.
3. The device is now registered and will be given an `api_key` when it first calls `/api/setup`.

Once added, you can refresh the **Devices** page to see it appear, along with
telemetry (battery, Wi-Fi signal, firmware version, last seen) that updates on
every poll.

Open the device to set a friendly **name**, a **refresh rate** (seconds between
polls), and which **playlist** it should show.

By default a newly registered device has no playlist assigned, so it shows a
placeholder image until you assign one (step 6).

---

## 5. Build a screen and a playlist

In the admin UI:

1. **Screens → create**: pick a plugin type (Clock, Weather, Static Image),
   give it a name, then open it to edit its settings JSON and click
   **Render preview** to see the 1-bit result. For a static image, use the
   upload form. See the [Plugins reference](PLUGINS.md) for each plugin's
   settings.
2. **Playlists → create**: name a playlist, then add one or more screens to it.
   A device cycles through a playlist's screens in order, one per poll.

(The seeded **Example** playlist already contains a Clock and a Weather screen
if you'd rather start from that.)

---

## 6. Assign the playlist to your device

Open the device under **Devices**, choose the playlist, and **Save**. On its
next poll the device will fetch and render the first screen, advancing through
the playlist on subsequent polls.

To force a re-render (e.g. after editing a screen), use **Force refresh** on the
device page — it clears the cached render so the next poll regenerates it.

---

## How the refresh cycle works

The firmware is **pull-based** and battery-optimized:

1. The device wakes from deep sleep and calls `GET /api/display` with telemetry
   headers (battery, Wi-Fi, firmware version, dimensions).
2. `go-trmnl` picks the device's next playlist screen, renders it to a
   800×480 1-bit image (cached by content hash), and returns the image URL plus
   a `refresh_rate`.
3. The device downloads and displays the image, then deep-sleeps for
   `refresh_rate` seconds and repeats.

So changes you make appear on the device on its **next wake**, not instantly.
Lower the device's refresh rate for faster updates (at the cost of battery).

---

## Customizing fonts

You can override the default Go fonts with your own TTF/OTF files, or choose between built-in TRMNL font bundles. The server uses a priority hierarchy: **Custom Overrides > Font Bundles > Go Defaults**.

### Font Bundles

TRMNL Framework 3.1+ supports two primary font bundles:

- **Classic**: The original look using *BlockKie*, *NicoBold*, and *Inter*.
- **TRMNL**: Custom hand-made pixel fonts optimized for e-ink.

You can select the bundle on a per-device basis in **Admin → Devices → (Device) → Settings**. These bundles look for specific filenames in `data/uploads/assets/bundles/{classic,trmnl}/`.

To see the effect of a different font bundle:
1. Go to **Admin → Screens** and select a screen.
2. In the **Preview** section, use the dropdown to select a device.
3. Click **Render preview**. The preview will now be rendered using the font bundle configured for that device.

**Classic bundle** expects:
- `Inter.ttf` (Sans and Mono)
- `BlockKie.ttf` (Title)

**TRMNL bundle** expects:
- `TRMNL21-Regular.ttf` (Sans and Mono)
- `TRMNL21-Bold.ttf` (Title)

### Custom Overrides (Global)

These settings apply to all devices and take precedence over font bundles.

1. **Obtain fonts**: You can use any TTF or OTF font. For the official TRMNL look, you can download the font bundles from the [Framework Releases](https://trmnl.com/framework/releases) page.
    - [TRMNL Font Bundle](https://trmnl.com/fonts/bundles/trmnl/trmnl-fonts-bundle--trmnl.zip) (Recommended for e-ink)
    - [Classic Font Bundle](https://trmnl.com/fonts/bundles/classic/trmnl-fonts-bundle--classic.zip)
2. **Upload fonts**: Put your font files (e.g., `Inter-Regular.ttf`) into the `data/uploads/assets/` directory of your `go-trmnl` installation.
3. **Configure in Admin UI**:
    - Go to **Settings** in the Admin UI.
    - Enter the filename of your uploaded font in the corresponding field:
        - **Sans-serif font**: Used for most standard text.
        - **Monospace font**: Used for code, data, or technical information.
        - **Title font**: Used for headers and bold text.
4. **Save**: Click **Save**.

### Automatic Defaults

If no custom fonts are provided and no bundle assets are found in `data/uploads/assets/bundles/`, the server falls back to built-in Go fonts.

## Testing screens without a device

You don't need a registered device (or even the server) to see what a screen
will look like. Three options, from least to most setup:

**1. Render a screen straight to image files (no server, no database).**
The `trmnl-render` dev tool runs a plugin and writes 1-bit `.bmp` and `.png`
files you can open locally:

```sh
go run ./cmd/trmnl-render -list
go run ./cmd/trmnl-render -plugin clock -settings '{"use_24h":true,"label":"Office"}' -out clock
go run ./cmd/trmnl-render -plugin weather -settings '{"location":"Berlin"}' -dither threshold -out weather
```

Flags: `-plugin`, `-settings` (JSON), `-out`, `-dither`
(`floyd_steinberg`/`threshold`), `-assets` (dir for the static-image plugin),
`-width`/`-height`. This is the quickest way to iterate on a plugin or settings.

**2. Preview in the admin UI (no device needed).**
Create a screen under **Admin → Screens**, edit its settings, and click
**Render preview**. The rendered PNG is shown on the page and served at
`/uploads/<hash>.png`.

**3. Simulate a real device over HTTP.**
Because devices must be pre-registered, you must first add the MAC to the server
via the admin UI (step 4). Then you can exercise the full display path
(telemetry, playlist selection, caching):

```sh
BASE=http://localhost:8080
MAC=AA:BB:CC:DD:EE:FF

# Grab the api_key for the pre-registered MAC.
KEY=$(curl -s -H "ID: $MAC" $BASE/api/setup | sed -n 's/.*"api_key":"\([^"]*\)".*/\1/p')

# Assign the seeded "Example" playlist to this simulated device in the admin UI
# (Devices -> the new device -> Playlist), then fetch its display:
curl -s -H "ID: $MAC" -H "Access-Token: $KEY" $BASE/api/display

# Download the image it points at:
curl -s -o screen.bmp "$BASE/uploads/$(curl -s -H "ID: $MAC" -H "Access-Token: $KEY" $BASE/api/display | sed -n 's/.*"filename":"\([^"]*\)".*/\1/p').bmp"
```

(Until you assign a non-empty playlist to the simulated device, `/api/display`
returns the placeholder image — see [Device API reference](API.md).)

## Configuration

All settings are flags or environment variables (flags win):

| Flag                | Env                      | Default               | Purpose                                                                                  |
|---------------------|--------------------------|-----------------------|------------------------------------------------------------------------------------------|
| `-listen`           | `TRMNL_LISTEN`           | `:8080`               | HTTP listen address                                                                      |
| `-base-url`         | `TRMNL_BASE_URL`         | auto (LAN IP)         | Public URL the device uses                                                               |
| `-data-dir`         | `TRMNL_DATA_DIR`         | `./data`              | Root for the database and uploads                                                        |
| `-db`               | `TRMNL_DB`               | `<data-dir>/trmnl.db` | SQLite database path                                                                     |
| `-uploads`          | `TRMNL_UPLOADS`          | `<data-dir>/uploads`  | Rendered image directory                                                                 |
| `-admin-user`       | `TRMNL_ADMIN_USER`       | `admin`               | Admin UI username                                                                        |
| `-admin-password`   | `TRMNL_ADMIN_PASSWORD`   | (empty)               | Admin UI password; empty disables auth                                                   |
| `-cleanup-interval` | `TRMNL_CLEANUP_INTERVAL` | `1h`                  | How often to prune the image cache; `0` disables                                         |
| `-log-retention`    | `TRMNL_LOG_RETENTION`    | `32d`                 | How long to keep device logs (e.g. `32d`, `720h`); `0` disables                          |
| `-secret-key`       | `TRMNL_SECRET_KEY`       | auto-generated        | Key to encrypt stored credentials at rest; default is a key generated under the data dir |
| `-no-encryption`    | `TRMNL_NO_ENCRYPTION`    | `false`               | Store credentials in plaintext instead of encrypting them                                |

Dithering mode (Floyd-Steinberg vs. threshold) is set in **Admin → Settings**.
Google OAuth clients and calendar accounts are configured entirely in
**Admin → Calendar**; see the
[Family Calendar plugin](plugins/familycalendar.md) docs.

### Credential encryption

Sensitive credentials stored in the database are **encrypted at rest by default**
(AES-256-GCM, key derived with scrypt). This is a general mechanism; the calendar
plugin (OAuth tokens, CalDAV passwords, OAuth client secrets) is the first user.
On first start, if you have not set `-secret-key` / `TRMNL_SECRET_KEY`, a random
key is generated and saved to `<data-dir>/secret.key` (mode `0600`), logged once.
**Back up that key (or set your own):** if it is lost, the encrypted values cannot
be read and must be re-entered. Provide your own key with `-secret-key` to keep it
outside the data dir. To store credentials in plaintext instead, pass
`-no-encryption` (a warning is logged). Existing plaintext entries are encrypted
automatically on the next start once a key is in effect.

### Rendered-image cache

Rendered screens are written to the uploads directory as content-hashed
`<hash>.bmp` / `<hash>.png` files and served from there. Because some screens
re-render often (the clock changes every minute), the server periodically prunes
this cache: every `cleanup-interval` it removes image files that are no longer
referenced by any screen and are older than one interval (a grace window).
Uploaded static-image assets (under `uploads/assets/`) and the placeholder are
never pruned. Removal is safe — if a still-needed image is pruned, the next poll
re-renders it (the file name is deterministic from its content). Set
`-cleanup-interval 0` to disable pruning.

### Device logs

Devices post log entries to `/api/log`; these accumulate in the database. The
same maintenance loop prunes entries older than `-log-retention` (default 32
days). Set `-log-retention 0` to keep logs indefinitely.

---

## Troubleshooting

**The device registers but shows the placeholder.**
It has no playlist assigned, or the playlist is empty. Assign a non-empty
playlist on the device page (step 6).

**The device can't fetch the image / nothing shows.**
The `-base-url` must be the host's **LAN IP**, reachable by the device, not
`127.0.0.1`/`localhost`. Confirm from another LAN machine:
`curl http://<your-lan-ip>:8080/healthz` should return `ok`.

**Edits don't appear.**
The device only updates on its next wake (every `refresh_rate` seconds). Use
**Force refresh**, and lower the refresh rate if you want quicker updates.

**A screen looks wrong / too dark.**
The panel is 1-bit (pure black & white). Try the other dithering mode in
**Settings**: Floyd-Steinberg suits photos/gradients, Threshold suits
text/sharp UI. For more on preparing images for e-ink, see the official
[image preparation guide](https://docs.trmnl.com/go/diy/imagemagick-guide)
(go-trmnl does the conversion in pure Go, but the principles are the same).

**Device-side / firmware issues** (Wi-Fi onboarding, setting the custom server
URL, flashing): those are on the device, not this server — see
[enable Wi-Fi pairing mode](https://help.trmnl.com/en/articles/11511577-enable-wifi-pairing-mode),
[connect to a BYOS server](https://help.trmnl.com/en/articles/12263392-connect-your-device-to-terminus-byos),
the [BYOS overview](https://docs.trmnl.com/go/diy/byos), and the
[firmware repo](https://github.com/usetrmnl/firmware).
