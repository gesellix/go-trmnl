# go-trmnl

[![CI](https://github.com/gesellix/go-trmnl/actions/workflows/ci.yml/badge.svg)](https://github.com/gesellix/go-trmnl/actions/workflows/ci.yml)

A self-hosted **BYOS** (Build Your Own Server) for the [TRMNL](https://usetrmnl.com)
e-ink display, written in Go. It replaces TRMNL's hosted backend so a device on
your LAN can be provisioned, fetch screens, and report telemetry without their
servers in the middle — your data and calendars stay on your own machine.

The firmware is **pull-based**: the device wakes from deep sleep, calls the
server, renders the returned 800x480 1-bit image, then sleeps for its
`refresh_rate` before polling again. The server never pushes to the device.

## What you can do

- **Auto-provision devices.** Point a TRMNL at `<base-url>` and it registers
  itself on its first call — no manual key entry.
- **Build screens from plugins** and arrange them into **playlists** that the
  device cycles through (one playlist per device, advanced each poll; multiple
  devices, each with its own position).
- **Manage everything in a web admin UI** (`/admin`): devices and telemetry,
  screens and live previews, playlists, calendar accounts, device logs, and
  settings.
- **Show a family calendar** merged from multiple **Google** and **Apple
  iCloud / CalDAV** accounts, deduplicated across people, with credentials
  encrypted at rest.
- Run it as a **single static binary** (or one small container) on anything
  down to a Raspberry Pi.

### Built-in screens

See the [plugins reference](docs/PLUGINS.md) for settings and examples.

| Plugin            | Type             | What it shows                                     |
|-------------------|------------------|---------------------------------------------------|
| Clock             | `clock`          | Time, date, optional label                        |
| Weather           | `weather`        | Current conditions + Today/Tomorrow (Open-Meteo)  |
| Family Calendar   | `familycalendar` | Merged agenda from Google + Apple/CalDAV accounts |
| Days Left in Year | `days_left_year` | Year-progress numbers + a dot grid                |
| Quote             | `quote`          | A quotation from a selectable provider            |
| Static Image      | `staticimage`    | An uploaded image scaled to the panel             |

Screens render to 800x480 and are reduced to 1-bit (black & white) with a
choice of dithering (Floyd-Steinberg or threshold), global or per screen.

## Quick start

```sh
# From source:
go run ./cmd/trmnld -base-url http://<your-lan-ip>:8080 -admin-password changeme

# Or the released container:
docker run -p 8080:8080 -v trmnl-data:/data \
  -e TRMNL_BASE_URL=http://<your-lan-ip>:8080 \
  -e TRMNL_ADMIN_PASSWORD=changeme \
  ghcr.io/gesellix/go-trmnl:latest
```

Then open `http://<your-lan-ip>:8080/admin`, and set your TRMNL device's custom
server URL to `<base-url>` — it auto-registers on its first poll. Use a
**LAN-reachable** base URL (not `127.0.0.1`), since the device fetches images
from it.

Full walkthrough: **[Getting started](docs/GETTING-STARTED.md)**.

## Configuration

Flags or environment variables (flags win). The essentials:

| Flag              | Env                    | Default       | Purpose                                       |
|-------------------|------------------------|---------------|-----------------------------------------------|
| `-base-url`       | `TRMNL_BASE_URL`       | auto (LAN IP) | Public URL the device uses to reach the server |
| `-data-dir`       | `TRMNL_DATA_DIR`       | `./data`      | Root for the SQLite database and uploads      |
| `-admin-password` | `TRMNL_ADMIN_PASSWORD` | (empty)       | Admin UI password; empty disables auth        |
| `-secret-key`     | `TRMNL_SECRET_KEY`     | (empty)       | Encrypts calendar credentials at rest         |

The `/admin` UI uses HTTP Basic Auth when a password is set. Device endpoints
(`/api/*`) and `/uploads` are unauthenticated, since the firmware cannot supply
credentials. See [Getting started](docs/GETTING-STARTED.md#configuration) for
the complete list (listen address, DB/uploads paths, cleanup and log-retention
intervals, etc.).

## Documentation

- **[Getting started](docs/GETTING-STARTED.md)** — run the server, point a
  device at it, build screens and playlists, full config reference. Start here.
- **[Plugins reference](docs/PLUGINS.md)** — built-in screens, their settings,
  and how to add your own. Calendar setup (Google OAuth, Apple/CalDAV) is in the
  [Family Calendar page](docs/plugins/familycalendar.md).
- **[Device API reference](docs/API.md)** — the firmware-facing endpoints,
  headers and responses.
- **[Design notes](docs/PLAN.md)** — architecture and how the server is built.
- **[Roadmap](ROADMAP.md)** — non-binding notes on possible future work.

Official TRMNL references: [BYOS overview](https://docs.trmnl.com/go/diy/byos),
[DIY introduction](https://docs.trmnl.com/go/diy/introduction),
[firmware](https://github.com/usetrmnl/firmware).

## Design goals

- **Pure Go, no CGO.** SQLite via [`modernc.org/sqlite`](https://modernc.org/sqlite);
  images drawn with [`fogleman/gg`](https://github.com/fogleman/gg) and SVG
  rasterization — no ImageMagick, no headless browser. Builds statically and
  cross-compiles to ARM / Raspberry Pi.
- **Single binary.** HTML templates, static assets, migrations and the timezone
  database are embedded.

## Development

```sh
make build      # static binary into ./build
make test       # go test -race with coverage
make lint       # golangci-lint
make vuln       # govulncheck

# Render a screen to image files without a server or device (plugin dev aid):
go run ./cmd/trmnl-render -plugin weather -settings '{"location":"Berlin"}' -out weather
```

See [Testing screens without a device](docs/GETTING-STARTED.md#testing-screens-without-a-device)
for previewing screens via the CLI, the admin UI, or a simulated device.

CI runs tests (`-race`), `golangci-lint`, a cross-platform build matrix,
`govulncheck`, a Docker build and CodeQL. Pushing a `vX.Y.Z` tag triggers the
release workflow (binaries + checksums, a GitHub release, and a multi-arch image
on GHCR). See [`.github/workflows/`](.github/workflows/).
