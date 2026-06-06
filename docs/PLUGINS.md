# Plugins reference

A **screen** is an instance of a **plugin** plus a small JSON settings object.
You create screens in **Admin → Screens**, edit their settings JSON there, and
click **Render preview** to see the 1-bit result. Screens are arranged into
**playlists**, which are assigned to devices.

All plugins render to an 800×480 image that is then reduced to 1-bit (black &
white) using the dithering mode from **Admin → Settings** (Floyd-Steinberg or
threshold).

---

## Clock

Type `clock`. Shows the current time, date and an optional label. No network
access.

**Settings**

| Key        | Type   | Default       | Notes                                          |
|------------|--------|---------------|------------------------------------------------|
| `use_24h`  | bool   | `false`       | 24-hour (`14:08`) vs 12-hour (`2:08 PM`)       |
| `timezone` | string | server local  | IANA name, e.g. `Europe/Berlin`, `America/New_York` |
| `label`    | string | (empty)       | Small heading above the time                   |

```json
{ "use_24h": true, "timezone": "Europe/Berlin", "label": "Office" }
```

Refresh hint: ~1 minute.

---

## Weather

Type `weather`. Shows current conditions plus a 4-day forecast with simple
1-bit weather icons. Data comes from the free, key-less
[Open-Meteo](https://open-meteo.com) API. The screen is modeled on the official
[TRMNL Weather plugin](https://help.trmnl.com/en/articles/10033272-weather).

**Settings**

| Key         | Type   | Default  | Notes                                                    |
|-------------|--------|----------|----------------------------------------------------------|
| `location`  | string | (empty)  | City name; geocoded when `latitude`/`longitude` are unset |
| `latitude`  | number | `0`      | Preferred when non-zero                                  |
| `longitude` | number | `0`      | Preferred when non-zero                                  |
| `units`     | string | `metric` | `metric` (°C, km/h) or `imperial` (°F, mph)             |
| `label`     | string | `Weather`| Shown in the corner                                      |

```json
{ "location": "Berlin", "units": "metric", "label": "Weather" }
```

```json
{ "latitude": 52.52, "longitude": 13.41, "units": "imperial" }
```

Refresh hint: ~30 minutes.

---

## Static Image

Type `staticimage`. Displays an uploaded image, scaled to fit the panel
(letterboxed on white). PNG, JPEG, GIF, BMP and WebP are accepted.

Use the **Upload image** form on the screen's page — it stores the file and
fills in the settings for you. The settings just point at the stored asset:

```json
{ "file": "a1b2c3d4e5f6a7b8.png" }
```

Refresh hint: ~24 hours (static content changes rarely).

---

## Special functions (per device)

Separate from screens, each device has a **special function** field
(**Devices → device → Settings**) returned to the firmware in
`/api/display`. The available values are `none`, `identify`, `sleep`,
`add_wifi`, `restart_playlist`, `rewind`, `send_to_me`. How the device acts on
each is defined by the firmware — see the
[firmware repo](https://github.com/usetrmnl/firmware). `go-trmnl` simply passes
the configured value through.

---

## Writing a new plugin

Plugins implement the `plugins.Plugin` interface (`internal/plugins/plugin.go`):
`Type`, `Title`, `DataModel`, `Render`, `DefaultRefresh`. `DataModel` does any
IO (e.g. an API call) and is kept separate from `Render` (which draws to an
`*image.RGBA`) so rendering can be tested without network. Register the plugin
from an `init()` in its package and blank-import that package in
`cmd/trmnld/main.go`. The built-in `clock` plugin is the simplest example to
copy.
