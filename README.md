# go-trmnl

A self-hosted **BYOS** (Build Your Own Server) for the [TRMNL](https://usetrmnl.com)
e-ink display, written in Go. It replaces TRMNL's hosted backend so a device on
your LAN can be provisioned, fetch screens, and report telemetry without their
servers in the middle.

The TRMNL firmware is **pull-based**: the device wakes from deep sleep, calls the
server, renders the returned 800x480 1-bit image, then sleeps for `refresh_rate`
seconds. The server never pushes to the device.

## Status

See [`docs/PLAN.md`](docs/PLAN.md) for the full design and milestone breakdown.

- [x] **M0** Skeleton: config, SQLite store + migrations, HTTP server
- [x] **M1** Device API: `/api/setup`, `/api/display`, `/api/log`, `/uploads`
- [x] **M2** Render pipeline: 1-bit BMP3 + PNG with Floyd-Steinberg dithering
- [x] **M3** Plugins + playlists (built-in clock and static-image plugins)
- [x] **M4** Admin web UI: devices, screens, playlists, logs, settings

The core BYOS platform is functional end-to-end: a device auto-registers, an
operator builds screens and playlists in the admin UI, and the device renders
them. Remaining polish (M5): admin auth, OTA firmware fields, more plugins.

## Admin UI

Browse to `<base-url>/admin` to manage devices, build screens from plugins,
arrange them into playlists, assign a playlist to a device, view device logs,
and preview rendered screens.

## Design goals

- **Pure Go, no CGO.** SQLite via [`modernc.org/sqlite`](https://modernc.org/sqlite),
  image rendering via [`fogleman/gg`](https://github.com/fogleman/gg) and SVG
  rasterization, no ImageMagick and no headless browser. Builds statically and
  cross-compiles to ARM / Raspberry Pi.
- **Single binary.** Templates and migrations are embedded.

## Device protocol

All device requests carry an `ID: <MAC>` header. Image URLs must use a
LAN-reachable base URL (not `127.0.0.1`).

| Method | Path           | Purpose                                                        |
|--------|----------------|----------------------------------------------------------------|
| GET    | `/api/setup`   | Auto-provision an unknown MAC; returns `api_key`, `friendly_id`|
| GET    | `/api/display` | Persist telemetry, return the next screen image URL            |
| POST   | `/api/log`     | Store device log entries (returns 204)                         |
| GET    | `/uploads/*`   | Serve rendered images to the device                            |

## Running

```sh
go run ./cmd/trmnld -base-url http://<your-lan-ip>:8080
```

Configuration (flags or environment variables; flags win):

| Flag         | Env                | Default               | Purpose                              |
|--------------|--------------------|-----------------------|--------------------------------------|
| `-listen`    | `TRMNL_LISTEN`     | `:8080`               | HTTP listen address                  |
| `-base-url`  | `TRMNL_BASE_URL`   | auto (LAN IP)         | Public URL the device uses           |
| `-data-dir`  | `TRMNL_DATA_DIR`   | `./data`              | Root for the database and uploads    |
| `-db`        | `TRMNL_DB`         | `<data-dir>/trmnl.db` | SQLite database path                 |
| `-uploads`   | `TRMNL_UPLOADS`    | `<data-dir>/uploads`  | Rendered image directory             |

Point your TRMNL device's custom server URL at `<base-url>` and it will
auto-register on its first `/api/setup` call.

## Development

```sh
go build ./...
go test ./... -race
```
