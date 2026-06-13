# Device API reference

These are the firmware-facing endpoints `go-trmnl` implements. They follow the
TRMNL BYOS contract so the stock firmware can talk to this server unchanged.

For the canonical, device-side definition of this protocol see the official
references:

- [BYOS overview](https://docs.trmnl.com/go/diy/byos)
- [Private API introduction](https://docs.trmnl.com/go/private-api/introduction)
- [Firmware](https://github.com/usetrmnl/firmware) and the reference server
  [Terminus / byos_hanami](https://github.com/usetrmnl/byos_hanami)

All device requests send an `ID` header containing the device's MAC address.
Image URLs returned by the server must be reachable by the device on the LAN.
None of these endpoints are authenticated by username/password — the device is
identified by its MAC (`ID`) and, after setup, its `Access-Token`.
Authentication can be disabled globally with `-no-device-auth`.

---

## `GET /api/setup`

Returns the credentials for a pre-registered device. Unknown devices are not
automatically provisioned and return a `404 Not Found`. Calling it again
for a registered MAC returns the existing credentials (idempotent).

**Request headers**

| Header       | Required | Notes                                 |
|--------------|----------|---------------------------------------|
| `ID`         | yes      | MAC address, e.g. `AA:BB:CC:DD:EE:FF` |
| `FW-Version` | no       | Firmware version, stored as telemetry |
| `Model`      | no       | Hardware model, stored as telemetry   |

**Response `200`**

```json
{
  "status": 200,
  "api_key": "OScdcN0kFbKjFcid9Kz6Cx",
  "friendly_id": "A1B2C3",
  "image_url": "http://192.168.1.10:8080/uploads/placeholder.bmp",
  "filename": "placeholder",
  "message": "Welcome to go-trmnl!"
}
```

A missing or malformed `ID` header returns `422` with an RFC 9457
problem-details body.

---

## `GET /api/display`

Persists telemetry from the request headers, selects the device's next playlist
screen, ensures a rendered 800×480 1-bit image exists, and returns it.

**Request headers**

| Header                               | Required | Notes                             |
|--------------------------------------|----------|-----------------------------------|
| `ID`                                 | yes      | MAC address                       |
| `Access-Token`                       | yes      | The `api_key` returned from setup |
| `FW-Version`                         | no       | Firmware version                  |
| `Model`                              | no       | Hardware model                    |
| `Battery-Voltage`                    | no       | Volts, float                      |
| `RSSI`                               | no       | Wi-Fi signal, dBm                 |
| `Width`/`Height`                     | no       | Panel dimensions                  |
| `Refresh-Rate`                       | no       | Last refresh rate the device used |
| `Battery-Charging` / `USB-Connected` | no       | Charging state                    |
| `WiFi-Status`                        | no       | e.g. `connected`                  |

**Response `200`**

```json
{
  "status": 0,
  "image_url": "http://192.168.1.10:8080/uploads/<hash>.bmp",
  "filename": "<hash>",
  "refresh_rate": 900,
  "update_firmware": false,
  "firmware_url": null,
  "reset_firmware": false,
  "special_function": "none",
  "image_url_timeout": 0,
  "temperature_profile": "default"
}
```

| Field                 | Meaning                                                        |
|-----------------------|----------------------------------------------------------------|
| `status`              | `0` = OK                                                       |
| `image_url`           | URL of the 1-bit BMP to display                                |
| `filename`            | Content hash of the image (cache-busting stem)                 |
| `refresh_rate`        | Seconds to deep-sleep before the next poll                     |
| `update_firmware`     | `true` when an OTA update is queued for this device            |
| `firmware_url`        | Firmware binary URL when `update_firmware` is true, else null  |
| `reset_firmware`      | One-shot firmware reset flag                                   |
| `special_function`    | e.g. `none`, `identify`, `sleep` (see [Plugins](PLUGINS.md))   |
| `image_url_timeout`   | Image fetch timeout hint (ms)                                  |
| `temperature_profile` | Display temperature profile                                    |

`update_firmware` and `reset_firmware` are **one-shot**: the server clears them
after serving once, to avoid boot loops.

### Authentication & Pre-registration

- **Pre-registration:** Devices must be manually added by MAC address in the
  Admin UI (**Devices → Add by MAC address**) before their first contact.
- **Access-Token:** By default, a valid `Access-Token` (the `api_key` from setup)
  is required for `/api/display`.
- **Disabling Auth:** Start `trmnld` with `-no-device-auth` (or set
  `TRMNL_NO_DEVICE_AUTH=1`) to skip token validation entirely. This is useful
  for simple BYOS setups where network security is handled elsewhere.

The matching image is also served at `GET /uploads/<hash>.bmp` (and a `.png` of
the same render, used for previews in the admin UI).

---

## `POST /api/log`

Stores a batch of device log entries. An unknown device is rejected.

**Request headers:** `ID` (MAC address).

**Request body**

```json
{
  "logs": [
    {
      "id": 1,
      "message": "boot",
      "created_at": 1700000000,
      "wifi_status": "connected",
      "wifi_signal": -54,
      "battery_voltage": 4.0,
      "wake_reason": "timer",
      "firmware_version": "1.5.2",
      "source_path": "src/bl.cpp",
      "source_line": 42,
      "refresh_rate": 900,
      "sleep_duration": 900,
      "free_heap_size": 100000,
      "max_alloc_size": 50000,
      "special_function": "none"
    }
  ]
}
```

All fields are optional except that the request must be valid JSON. Entries are
stored against the device and shown under **Devices → Logs** in the admin UI.
An unparsable body returns `422`.

---

## `GET /uploads/<name>`

Static serving of rendered images (`<hash>.bmp`, `<hash>.png`,
`placeholder.bmp`). This is where the device fetches the image referenced by
`image_url`. Not authenticated.
