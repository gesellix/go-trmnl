# go-trmnl template

Adds the go-trmnl server to the stack: a scrape job, alert rules for battery
and silent devices, and a Grafana dashboard.

## Install

```sh
./add-template.sh trmnl
```

Then add the target to the stack's `scrape.env` (copy `scrape.env.example` if
it does not exist yet):

```sh
TRMNL_TARGET=192.168.1.10:8080
TRMNL_METRICS_USER=metrics
TRMNL_METRICS_PASSWORD=the-password-from-trmnld
```

`TRMNL_TARGET` is host:port only, no scheme and no `/metrics`. Use the address
the monitoring host reaches trmnld under; if both run on the same machine in
Docker, that is the host IP, not `localhost`.

The credentials are the ones trmnld was started with (`-metrics-user` /
`-metrics-password`, or `TRMNL_METRICS_USER` / `TRMNL_METRICS_PASSWORD` in
`/etc/trmnld/trmnld.env`). If you run `/metrics` without a password, delete the
`basic_auth` block from `victoriametrics/conf.d/trmnl.yml`.

Apply:

```sh
docker compose up -d            # picks up the new scrape.env
docker compose restart vmalert  # loads the new rules
```

Check <http://localhost:8428/targets>: the `trmnl` job should be `UP` within a
minute. The dashboard appears in Grafana as **TRMNL devices**.

## What it adds

| File                            | Goes to                   | Purpose                                       |
|---------------------------------|---------------------------|-----------------------------------------------|
| `scrape/trmnl.yml`              | `victoriametrics/conf.d/` | Scrapes `/metrics` every 60s with Basic Auth  |
| `rules/trmnl.yml`               | `vmalert/rules/`          | Battery, silence, WiFi and server-down alerts |
| `dashboards/trmnl-devices.json` | `grafana/dashboards/`     | Battery, signal, charging and a device table  |

## Alerts

| Alert                  | Fires when                                                | Severity |
|------------------------|-----------------------------------------------------------|----------|
| `TrmnlBatteryLow`      | 6h average below 3.4 V for an hour                        | warning  |
| `TrmnlBatteryCritical` | 6h average below 3.3 V                                    | critical |
| `TrmnlBatteryTrend`    | Extrapolation says empty within a week, while discharging | warning  |
| `TrmnlSilent`          | Three missed polls in a row                               | warning  |
| `TrmnlWifiWeak`        | 6h average RSSI below -80 dBm                             | warning  |
| `TrmnlServerDown`      | The scrape fails for 10 minutes                           | critical |

Not every firmware reports charging state (`Battery-Charging` /
`USB-Connected`). Where it is missing, `trmnl_battery_charging` is absent, the
dashboard's charging panel stays empty, and `TrmnlBatteryTrend` behaves as if
the device were discharging.

The battery thresholds are a starting point. Voltage swings with load and
temperature, and the discharge curve is flat for weeks before it drops, so
watch the dashboard for a few weeks and move the numbers to where they
actually mean something for your device. Editing `vmalert/rules/trmnl.yml`
directly is fine; `add-template.sh` will not overwrite it unless you pass `-f`.

The metrics and their meaning are documented in
[docs/MONITORING.md](../../../../docs/MONITORING.md).
