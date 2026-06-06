# Static Image plugin

Type: `staticimage`. Displays an uploaded image, scaled to fit the panel and
letterboxed on white, then dithered to 1-bit. Accepts **PNG, JPEG, GIF, BMP and
WebP**.

[← All plugins](../PLUGINS.md)

## Settings

| Key    | Type   | Notes                                                   |
|--------|--------|---------------------------------------------------------|
| `file` | string | File name within the assets directory (base name only)  |

```json
{ "file": "a1b2c3d4e5f6a7b8.png" }
```

The image is **not** referenced by an arbitrary path: only the base name is
used, and the file must live in the assets directory — `<data-dir>/uploads/assets/`
for the running server, or the `-assets` directory for the CLI. This keeps
lookups scoped to that folder.

Cache TTL hint: ~24 hours (static content changes rarely).

## How to test

### Admin UI (normal flow)

1. **Screens → create**, choose **Static Image**.
2. On the screen page, use the **Upload image** form. It stores the file under
   `<data-dir>/uploads/assets/` and fills in the `file` setting for you.
3. Click **Render preview**; optionally add the screen to a playlist and assign
   it to a device.

### CLI (no server or device)

Point `-assets` at a folder containing your image; `file` names it within that
folder:

```sh
mkdir -p pics && cp ~/Pictures/photo.jpg pics/photo.jpg
go run ./cmd/trmnl-render -plugin staticimage \
  -assets ./pics -settings '{"file":"photo.jpg"}' -out out
# writes out.bmp and out.png — open out.png
```

Compare dithering modes — `-dither floyd_steinberg` (default, best for
photos/gradients) vs `-dither threshold` (sharper for high-contrast graphics).

See [Testing screens without a device](../GETTING-STARTED.md#testing-screens-without-a-device).

## Notes

- Scaling preserves aspect ratio: e.g. a 1200×800 source becomes 720×480
  centered, with white bars filling the remaining width/height.
- If `file` is unset or the file is missing, `DataModel` returns an error. On a
  device this surfaces as the placeholder image (the display handler falls back
  gracefully); with the CLI the error is printed.
