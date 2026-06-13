# Plugins

A **screen** is an instance of a **plugin** plus a small JSON settings object.
You create screens in **Admin → Screens**, edit their settings JSON there, and
click **Render preview** to see the 1-bit result. Screens are arranged into
**playlists**, which are assigned to devices.

All plugins render to an 800×480 image that is then reduced to 1-bit (black &
white) using the dithering mode from **Admin → Settings** (Floyd-Steinberg or
threshold). For ways to preview a screen without a device, see
[Testing screens without a device](GETTING-STARTED.md#testing-screens-without-a-device).

## Built-in plugins

| Plugin                                       | Type             | Summary                                                                 |
|----------------------------------------------|------------------|-------------------------------------------------------------------------|
| [Clock](plugins/clock.md)                    | `clock`          | Current time, date and an optional label                                |
| [Weather](plugins/weather.md)                | `weather`        | Current conditions + Today/Tomorrow forecast (Open-Meteo)               |
| [Static Image](plugins/staticimage.md)       | `staticimage`    | An uploaded image scaled to fit the panel                               |
| [Family Calendar](plugins/familycalendar.md) | `familycalendar` | Merged agenda from Google/Apple calendar accounts with weather forecast |
| [Days Left This Year](plugins/daysleft.md)   | `days_left_year` | Year-progress numbers + a dot grid of the year                          |
| [Quote](plugins/quote.md)                    | `quote`          | A quotation from a selectable provider                                  |

Each page documents that plugin's settings, JSON examples and how to test it.

## Special functions (per device)

Separate from screens, each device has a **special function** field
(**Devices → device → Settings**) returned to the firmware in `/api/display`.
The available values are `none`, `identify`, `sleep`, `add_wifi`,
`restart_playlist`, `rewind`, `send_to_me`. How the device acts on each is
defined by the firmware — see the
[firmware repo](https://github.com/usetrmnl/firmware). `go-trmnl` simply passes
the configured value through.

## Writing a new plugin

Plugins implement the `plugins.Plugin` interface (`internal/plugins/plugin.go`):
`Type`, `Title`, `DataModel`, `Render`, `DefaultRefresh`. `DataModel` does any
IO (e.g. an API call) and is kept separate from `Render` (which draws to an
`*image.RGBA`) so rendering can be tested without network. Register the plugin
from an `init()` in its package and blank-import that package in both
`cmd/trmnld/main.go` and `cmd/trmnl-render/main.go`. The built-in
[clock](plugins/clock.md) plugin is the simplest example to copy.

While developing, render your plugin straight to image files with the
`trmnl-render` CLI:

```sh
go run ./cmd/trmnl-render -plugin <type> -settings '<json>' -out out
```
