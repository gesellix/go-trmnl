# go-trmnl

[![CI](https://github.com/gesellix/go-trmnl/actions/workflows/ci.yml/badge.svg)](https://github.com/gesellix/go-trmnl/actions/workflows/ci.yml)

A self-hosted **BYOS** (Build Your Own Server) for the [TRMNL](https://usetrmnl.com)
e-ink display, written in Go. It replaces TRMNL's hosted backend so a device on
your LAN can be provisioned, fetch screens, and report telemetry without their
servers in the middle.

The TRMNL firmware is **pull-based**: the device wakes from deep sleep, calls the
server, renders the returned 800x480 1-bit image, then sleeps for `refresh_rate`
seconds. The server never pushes to the device.

## Documentation

- **[Getting started](docs/GETTING-STARTED.md)** — run the server, point a
  device at it, and build your first screen and playlist. Start here.
- **[Device API reference](docs/API.md)** — the firmware-facing endpoints,
  headers and responses.
- **[Plugins reference](docs/PLUGINS.md)** — built-in screens and their settings.
- **[Design & milestones](docs/PLAN.md)** — how the server is built.

Official TRMNL references: [BYOS overview](https://docs.trmnl.com/go/diy/byos),
[DIY introduction](https://docs.trmnl.com/go/diy/introduction),
[firmware](https://github.com/usetrmnl/firmware).

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

| Flag              | Env                    | Default               | Purpose                                |
|-------------------|------------------------|-----------------------|----------------------------------------|
| `-listen`         | `TRMNL_LISTEN`         | `:8080`               | HTTP listen address                    |
| `-base-url`       | `TRMNL_BASE_URL`       | auto (LAN IP)         | Public URL the device uses             |
| `-data-dir`       | `TRMNL_DATA_DIR`       | `./data`              | Root for the database and uploads      |
| `-db`             | `TRMNL_DB`             | `<data-dir>/trmnl.db` | SQLite database path                   |
| `-uploads`        | `TRMNL_UPLOADS`        | `<data-dir>/uploads`  | Rendered image directory               |
| `-admin-user`     | `TRMNL_ADMIN_USER`     | `admin`               | Admin UI username                      |
| `-admin-password` | `TRMNL_ADMIN_PASSWORD` | (empty)               | Admin UI password; empty disables auth |

The `/admin` UI is protected with HTTP Basic Auth when `-admin-password` (or
`TRMNL_ADMIN_PASSWORD`) is set. The device endpoints (`/api/*`) and `/uploads`
are never authenticated, since the device cannot supply credentials.

Point your TRMNL device's custom server URL at `<base-url>` and it will
auto-register on its first `/api/setup` call.

## Install

Tagged releases publish a multi-arch Docker image to GHCR and prebuilt binaries
(linux/macOS/windows/freebsd, incl. arm/arm64) with checksums on the
[releases page](https://github.com/gesellix/go-trmnl/releases).

```sh
# Released image (once a vX.Y.Z tag exists)
docker run -p 8080:8080 -v trmnl-data:/data \
  -e TRMNL_BASE_URL=http://<your-lan-ip>:8080 \
  -e TRMNL_ADMIN_PASSWORD=changeme \
  ghcr.io/gesellix/go-trmnl:latest
```

## Docker (from source)

```sh
docker build -t go-trmnl .
docker run -p 8080:8080 -v trmnl-data:/data \
  -e TRMNL_BASE_URL=http://<your-lan-ip>:8080 \
  -e TRMNL_ADMIN_PASSWORD=changeme \
  go-trmnl
```

The image is a static binary on `distroless/static` (CA certificates included
for the weather plugin; the timezone database is embedded in the binary).

## Development

```sh
make build      # static binary into ./build
make test       # go test -race with coverage
make lint       # golangci-lint
make vuln       # govulncheck
```

CI runs tests (with `-race`), `golangci-lint`, a cross-platform build matrix,
`govulncheck`, a Docker build, and CodeQL analysis. A scheduled Security
workflow runs `govulncheck`, `staticcheck` and Semgrep. Pushing a `vX.Y.Z` tag
triggers the Release workflow, which builds binaries + checksums, publishes a
GitHub release, and pushes a multi-arch image to GHCR. See `.github/workflows/`.

To cut a release:

```sh
git tag v0.1.0 && git push origin v0.1.0
```
