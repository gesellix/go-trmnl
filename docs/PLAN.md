# go-trmnl: a full-fledged BYOS server for TRMNL

## Context

The repo is greenfield (only `go.mod`, module `github.com/gesellix/go-trmnl`, Go 1.25). The goal is a self-hostable "Build Your Own Server" (BYOS) for the TRMNL e-ink display, replacing TRMNL's hosted backend so a real device on the LAN can be provisioned, fetch screens, and report logs/telemetry without their servers in the middle.

The TRMNL firmware uses a **pull model** (the device wakes from deep sleep, calls the server, renders the returned image, then sleeps for `refresh_rate` seconds). The server never pushes. The protocol is defined by the firmware (`usetrmnl/firmware`) and the canonical reference server (`usetrmnl/terminus` / `byos_hanami`).

Decisions locked with the user: **full platform** (device API + admin UI + playlists + plugin/screen rendering), **SQLite** (pure-Go `modernc.org/sqlite`, no CGO), **pure-Go image pipeline** (no ImageMagick, no headless browser), **chi** router. Because rendering must be pure Go (not Chromium), screens are drawn with a 2D canvas (`fogleman/gg`) and/or rasterized SVG (`srwiley/oksvg` + `rasterx`), then converted to 1-bit output.

## The device protocol to implement (firmware pulls; never push)

All device requests carry `ID: <MAC>` header. Image URLs must use a LAN-reachable base URL (not `127.0.0.1`).

1. **`GET /api/setup`** — headers `ID` (MAC, required), `FW-Version`, `Model`. Auto-provision an unknown MAC: create the device, generate `api_key` + `friendly_id`, return `200 {api_key, friendly_id, image_url, message}`. Malformed/missing `ID` -> `404`/`422` RFC-9457 problem-details JSON.
2. **`GET /api/display`** — headers `ID`, `Access-Token` (= api_key), `FW-Version`, `Battery-Voltage`, `RSSI`, `Width`, `Height`, `Refresh-Rate`, battery/charging headers, etc. Persist telemetry to the device row. Pick the next screen from the device's playlist, ensure a rendered 800x480 1-bit image exists, return `200 {status:0, image_url, filename, refresh_rate, update_firmware:false, firmware_url:null, reset_firmware:false, special_function:"none", image_url_timeout, temperature_profile}`. `filename` is a content hash (cache-busting). Bad token / unknown device -> `404` problem-details.
3. **`POST /api/log`** — header `ID`. Body `{logs:[{id, message, created_at(unix), wifi_status, sleep_duration, refresh_rate, free_heap_size, max_alloc_size, source_path, source_line, wake_reason, firmware_version, battery_voltage, special_function, wifi_signal}]}`. Insert log rows for the device, return `204`.
4. **`GET /uploads/*`** — static serving of rendered images, reachable by the device.

## Project layout

```
cmd/trmnld/main.go                 wire config -> store -> render -> chi server; graceful shutdown
internal/config/                   ListenAddr, PublicBaseURL, DataDir, DBPath, UploadsDir (flag + env)
internal/server/                   chi router assembly, middleware, lifecycle
internal/httpx/                    RFC-9457 problem-details writer, JSON helpers, header parsing
internal/store/                    *sql.DB, embedded numbered migrations, typed query methods
internal/device/                   provisioning (api_key/friendly_id), telemetry application
internal/playlist/                 NextScreen(device): round-robin via cursor column
internal/render/                   RGBA -> dither -> 1-bit BMP3 + PNG, content-hash filenames, disk cache
internal/plugins/                  Plugin interface + registry; built-ins clock/, staticimage/
internal/deviceapi/                firmware handlers: setup, display, log + MAC/token middleware
internal/admin/                    admin UI handlers (html/template)
web/templates/  web/static/        embedded admin templates + CSS/JS (embed.FS)
testdata/golden/                   golden BMP/PNG for renderer tests
```

`config` imports no other internal package. `store` hides SQL and returns domain structs. `deviceapi`/`admin` are thin HTTP layers over `device`/`playlist`/`render`.

## Database schema (SQLite)

Embedded numbered SQL migrations applied on startup via a `schema_migrations` tracker. DSN pragmas: `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`. Timestamps as Unix-second INTEGERs.

- **devices**: id, `mac` UNIQUE, `api_key` UNIQUE, `friendly_id` UNIQUE, name, model, fw_version, width(800), height(480), refresh_rate(900), playlist_id FK, playlist_cursor, battery_voltage, battery_charging, rssi, wifi_status, last_seen_at, created_at.
- **device_logs**: id, device_id FK CASCADE, log_id, message, created_at(unix), received_at, wifi_status, wifi_signal, sleep_duration, refresh_rate, free_heap_size, max_alloc_size, source_path, source_line, wake_reason, firmware_version, battery_voltage, special_function. Index `(device_id, received_at DESC)`.
- **plugins**: id, `type` (registry key), name, created_at — a configured plugin instance.
- **screens**: id, plugin_id FK CASCADE, name, settings_json, rendered_hash, rendered_at, created_at.
- **playlists**: id, name, created_at.
- **playlist_items**: id, playlist_id FK CASCADE, screen_id FK CASCADE, position, visible. Index `(playlist_id, position)`.
- **settings**: key PRIMARY KEY, value.

## chi routes

```go
r.Use(RequestID, RealIP, Recoverer, Logger)
r.Route("/api", func(r chi.Router){
  r.With(requireMACPresent).Get("/setup", h.Setup)        // provisions; no existing device required
  r.With(requireMAC, requireToken).Get("/display", h.Display)
  r.With(requireMAC).Post("/log", h.Log)                  // 204
})
r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))
r.Route("/admin", ...)                                    // dashboard, devices, screens, playlists, logs, settings
r.Handle("/static/*", http.FileServer(http.FS(staticFS)))
```

Middleware: `requireMAC` validates the `ID` header + loads the device into context (404 problem-details on miss); `requireToken` constant-time compares `Access-Token` to `api_key`; `setup` only checks the MAC is well-formed then provisions. Admin auth starts as a no-op stub (settings-driven later).

## Rendering pipeline (pure Go) — the riskiest piece

Per display request: `playlist.NextScreen(device)` -> plugin + settings; `plugin.Render(...)` returns an `*image.RGBA` (800x480) drawn via `gg` and/or rasterized SVG (`oksvg`+`rasterx`); `render.Process(img)` converts to 1-bit (threshold or **Floyd-Steinberg** error diffusion, ~30-line loop), computes `sha256` of the packed bytes as the filename, writes `<hash>.bmp` + `<hash>.png` under `UploadsDir` (skips if present), and returns `image_url = PublicBaseURL + "/uploads/<hash>.bmp"`. Re-render only when stale (TTL from `refresh_rate` / plugin hint), so the hot path is cheap.

**Custom 1-bit BMP3 encoder required** in `internal/render/bmp.go`: `golang.org/x/image/bmp.Encode` cannot emit 1-bpp. Write BITMAPFILEHEADER (14B) + BITMAPINFOHEADER (40B, `biBitCount=1`, `biClrUsed=2`) + 2-color table (black/white), all little-endian; **bottom-up scanlines** (row `height-1` first); **MSB-first** bit packing (leftmost pixel = bit 7); row stride `((width+31)/32)*4` (800px = 100 bytes, already 4-aligned). PNG output via `image/png` on an `image.Paletted` with a 2-color palette.

## Plugin / screen abstraction

```go
type RenderInput struct { Device device.Device; Screen store.Screen; Settings json.RawMessage; Now time.Time; Width, Height int }
type Plugin interface {
  Type() string
  DataModel(ctx, in) (any, error)        // IO/network, separated for testability
  Render(ctx, in, data) (*image.RGBA, error)
  DefaultRefresh() time.Duration         // cache TTL hint
}
// registry: Register(p) in init(); Get(type)
```

Built-ins: **clock** (zero IO, deterministic with fixed `Now`, embedded TTF via `gg` — also the M2 smoke screen) and **staticimage** (admin-uploaded image scaled to 800x480 via `x/image/draw.CatmullRom`, dithered). A **weather** plugin (JSON API + SVG icon) is a later stretch.

## Admin UI

`html/template` + chi + `embed.FS`, server-rendered POST/redirect/get, minimal vanilla JS (drag-reorder + live preview). Shared `layout.gohtml`. Pages: dashboard (counts, recent devices, render thumbnails), devices list/detail (telemetry + current PNG + "force refresh" = clear `rendered_hash`), screens CRUD (pick plugin type, edit settings, preview), playlists (reorder items, assign to devices), per-device logs (paginated), settings (base URL, default refresh, dithering toggle, admin auth).

## Configuration

`internal/config` via stdlib `flag` with env fallback: `-listen`/`TRMNL_LISTEN` (`:8080`), `-base-url`/`TRMNL_BASE_URL` (must be LAN-reachable, warn on loopback), `-data-dir`/`TRMNL_DATA_DIR` (`./data`), `-db` (`<data>/trmnl.db`), `-uploads` (`<data>/uploads`). Create dirs on startup.

## Dependencies (all pure Go, CGO disabled)

`github.com/go-chi/chi/v5`, `modernc.org/sqlite`, `github.com/fogleman/gg`, `github.com/srwiley/oksvg` + `github.com/srwiley/rasterx`, `golang.org/x/image`. Custom 1-bit BMP encoder uses only `encoding/binary`.

## Build order (runnable early)

- **M0** — skeleton: config, store open+migrate, chi server with `/healthz`.
- **M1** — device endpoints with a placeholder/clock image: MAC+token middleware, problem-details, `/api/setup` auto-provision, `/api/display`, `/api/log` -> 204, `/uploads/*`. A real TRMNL can now be pointed at the server.
- **M2** — real render pipeline: dither + custom 1-bit BMP3/PNG + content-hash cache; wire `display` to the clock plugin. Golden tests.
- **M3** — plugins + playlists: registry, clock + staticimage, `playlist.NextScreen` round-robin.
- **M4** — admin UI: read-only devices/logs first, then screens/playlists CRUD, then settings + force-refresh + upload.
- **M5** — polish: admin auth, firmware-update fields, `special_function`, weather plugin.

## Verification

- **Device endpoints** (`net/http/httptest` + temp sqlite, table-driven, `-race`): unknown MAC setup -> 200 + persisted device + stable api_key on repeat; missing `ID` -> 404/422 problem-details; `/api/display` good token -> `status==0`, `image_url` has base-URL prefix, `filename`==hash; bad token -> 404; `/api/log` -> 204 + rows inserted.
- **Renderer golden tests**: fixed `Now`/data -> render -> dither -> encode, byte-compare against `testdata/golden/*.{bmp,png}` (with a `-update-golden` switch). Explicit unit test asserting BMP header bytes: `biBitCount==1`, bottom-up first scanline, MSB-first packing, 4-byte stride.
- **Store tests** (`t.TempDir()` sqlite): UpsertDevice, InsertLogs, NextPlaylistItem cursor wraparound, FK cascade on device delete.
- **End-to-end manual**: `go run ./cmd/trmnld -base-url http://<lan-ip>:8080`, then `curl -H 'ID: AA:BB:CC:DD:EE:FF' http://localhost:8080/api/setup` (expect api_key/friendly_id), `curl -H 'ID: AA:BB:CC:DD:EE:FF' -H 'Access-Token: <key>' http://localhost:8080/api/display` (expect JSON + fetchable `image_url`), open the BMP, then point a real device at the base URL.
- `CGO_ENABLED=0 go build ./...` and `go test ./... -race` (static builds cross-compile to ARM / Raspberry Pi).
