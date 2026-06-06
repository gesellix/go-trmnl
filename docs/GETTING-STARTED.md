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

On the device side you set the **custom server / API base URL** to your
`go-trmnl` base URL (e.g. `http://192.168.1.10:8080`). The firmware appends
`/api/...` itself, so give it just the base.

How you enter that URL depends on your firmware/hardware (it's usually done in
the Wi-Fi captive portal during onboarding, or baked into a firmware build).
That part is device-specific, so follow the official instructions:

- [BYOS overview](https://docs.trmnl.com/go/diy/byos)
- [Firmware repo & build instructions](https://github.com/usetrmnl/firmware)

> Tip: you can simulate a device from any machine on the LAN with `curl` to
> confirm the server is reachable before involving hardware:
>
> ```sh
> curl -H "ID: AA:BB:CC:DD:EE:FF" http://<your-lan-ip>:8080/api/setup
> ```
>
> A JSON response containing `api_key` and `friendly_id` means it's working.
> See the [Device API reference](API.md) for all endpoints.

---

## 4. Watch it register

The first time the device calls `GET /api/setup`, `go-trmnl` **auto-registers**
it: it creates a device record and returns a generated `api_key` and a short
`friendly_id`. Refresh the admin **Devices** page and you'll see it appear,
along with telemetry (battery, Wi-Fi signal, firmware version, last seen) that
updates on every poll.

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

## Configuration

All settings are flags or environment variables (flags win):

| Flag              | Env                    | Default               | Purpose                                |
|-------------------|------------------------|-----------------------|----------------------------------------|
| `-listen`         | `TRMNL_LISTEN`         | `:8080`               | HTTP listen address                    |
| `-base-url`       | `TRMNL_BASE_URL`       | auto (LAN IP)         | Public URL the device uses             |
| `-data-dir`       | `TRMNL_DATA_DIR`       | `./data`              | Root for the database and uploads      |
| `-db`             | `TRMNL_DB`             | `<data-dir>/trmnl.db` | SQLite database path                   |
| `-uploads`        | `TRMNL_UPLOADS`        | `<data-dir>/uploads`  | Rendered image directory               |
| `-admin-user`     | `TRMNL_ADMIN_USER`     | `admin`               | Admin UI username                      |
| `-admin-password` | `TRMNL_ADMIN_PASSWORD` | (empty)               | Admin UI password; empty disables auth |

Dithering mode (Floyd-Steinberg vs. threshold) is set in **Admin → Settings**.

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
URL, flashing): those are on the device, not this server — see the official
[BYOS overview](https://docs.trmnl.com/go/diy/byos) and
[firmware repo](https://github.com/usetrmnl/firmware).
