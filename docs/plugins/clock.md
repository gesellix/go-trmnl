# Clock plugin

Type: `clock`. Shows the current time, date and an optional label. Performs no
network access, which makes it a good first screen and a deterministic test.

[← All plugins](../PLUGINS.md)

## Settings

| Key        | Type   | Default      | Notes                                               |
|------------|--------|--------------|-----------------------------------------------------|
| `use_24h`  | bool   | `false`      | 24-hour (`14:08`) vs 12-hour (`2:08 PM`)            |
| `timezone` | string | server local | IANA name, e.g. `Europe/Berlin`, `America/New_York` |
| `label`    | string | (empty)      | Small heading above the time                        |

```json
{ "use_24h": true, "timezone": "Europe/Berlin", "label": "Office" }
```

Cache TTL hint: ~1 minute (the rendered image is content-hashed, so it only
actually changes when the displayed minute does).

## How to test

Render straight to image files, no server or device needed:

```sh
go run ./cmd/trmnl-render -plugin clock \
  -settings '{"use_24h":true,"label":"Office"}' -out clock
# open clock.png
```

Or in the admin UI: **Screens → create (Clock) → edit settings → Render
preview**. See [Testing screens without a device](../GETTING-STARTED.md#testing-screens-without-a-device).

## Notes

- `timezone` must be a valid IANA name. An unknown/invalid value is ignored and
  the server's local time is used. The server binary embeds the timezone
  database (`time/tzdata`), so zones resolve even on minimal container images.
