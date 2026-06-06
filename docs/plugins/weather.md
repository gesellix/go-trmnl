# Weather plugin

Type: `weather`. Shows current conditions plus a 4-day forecast with simple
1-bit weather icons. Data comes from the free, key-less
[Open-Meteo](https://open-meteo.com) API. The layout is modeled on the official
[TRMNL Weather plugin](https://help.trmnl.com/en/articles/10033272-weather).

[← All plugins](../PLUGINS.md)

## Settings

| Key         | Type   | Default   | Notes                                                     |
|-------------|--------|-----------|-----------------------------------------------------------|
| `location`  | string | (empty)   | City name; geocoded when `latitude`/`longitude` are unset |
| `latitude`  | number | `0`       | Preferred when non-zero                                   |
| `longitude` | number | `0`       | Preferred when non-zero                                   |
| `units`     | string | `metric`  | `metric` (°C, km/h) or `imperial` (°F, mph)              |
| `label`     | string | `Weather` | Shown in the corner                                       |

```json
{ "location": "Berlin", "units": "metric", "label": "Weather" }
```

```json
{ "latitude": 52.52, "longitude": 13.41, "units": "imperial" }
```

Cache TTL hint: ~30 minutes.

## How to test

The plugin makes a live HTTP call to Open-Meteo, so the host needs internet
access. Render to files with no server or device:

```sh
# By city name (geocoded):
go run ./cmd/trmnl-render -plugin weather \
  -settings '{"location":"Berlin","units":"metric"}' -out weather

# By coordinates, imperial units:
go run ./cmd/trmnl-render -plugin weather \
  -settings '{"latitude":40.71,"longitude":-74.01,"units":"imperial"}' -out nyc
```

Or in the admin UI: **Screens → create (Weather) → edit settings → Render
preview**. See [Testing screens without a device](../GETTING-STARTED.md#testing-screens-without-a-device).

To test the rendering without network (e.g. in CI), the data fetch
(`DataModel`) is separated from drawing (`Render`); the unit tests in
`internal/plugins/weather/` mock Open-Meteo with an `httptest` server and also
render from a hand-built data model.

## Notes

- Provide either a `location` (geocoded via Open-Meteo) **or** explicit
  `latitude`/`longitude`. With neither set, the plugin returns an error (which
  shows as the placeholder image on a device).
- Icons are drawn as 1-bit silhouettes (sun, cloud, rain, snow, fog, storm)
  mapped from WMO weather codes.
