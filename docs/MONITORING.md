# Monitoring

trmnld exposes Prometheus metrics at `/metrics` (OpenMetrics text format), so
Prometheus, VictoriaMetrics/vmagent, Grafana Agent or anything else that speaks
the same protocol can scrape it. The endpoint is on by default.

The device values are read from the database at scrape time, not counted in
memory: they survive a restart and always reflect the last `/api/display` poll
of each device.

## A ready-made stack

If you have no monitoring yet, [`deploy/monitoring`](../deploy/monitoring/)
is a five-container Docker stack for a Raspberry Pi or NAS (VictoriaMetrics,
vmalert, Alertmanager, ntfy, Grafana) with a go-trmnl template that installs
the scrape job, the alert rules below and a dashboard:

```sh
cd deploy/monitoring
cp .env.example .env && $EDITOR .env
docker compose up -d
./add-template.sh trmnl
```

The rest of this page describes the endpoint itself, for an existing setup.

## Exposing the endpoint

By default `/metrics` is served on the regular listener(s), next to `/admin`
and `/api`, and it is **unauthenticated** (a warning is logged at startup).
Two things to decide:

**Credentials.** `-metrics-user` / `-metrics-password` guard the endpoint with
HTTP Basic Auth. They are deliberately separate from the admin credentials, so
a scraper never holds admin access:

```sh
trmnld -base-url http://192.168.1.10:8080 \
  -metrics-user scraper -metrics-password s3cret
```

**A separate port.** `-metrics-listen` moves `/metrics` to its own listener,
which then serves nothing else. Bound to loopback it is reachable only by a
scraper on the same host (no startup warning in that case):

```sh
trmnld -base-url http://192.168.1.10:8080 -metrics-listen 127.0.0.1:9090
```

On the shared listener every scrape shows up in the request log; the dedicated
listener does not log them. `-no-metrics` turns the endpoint off entirely.

| Flag                | Env                      | Default   | Purpose                                                 |
|---------------------|--------------------------|-----------|---------------------------------------------------------|
| `-no-metrics`       | `TRMNL_NO_METRICS`       | `false`   | Disable `/metrics`                                      |
| `-metrics-listen`   | `TRMNL_METRICS_LISTEN`   | (empty)   | Serve `/metrics` on its own address, e.g. `:9090`       |
| `-metrics-user`     | `TRMNL_METRICS_USER`     | `metrics` | Basic Auth username for `/metrics`                      |
| `-metrics-password` | `TRMNL_METRICS_PASSWORD` | (empty)   | Basic Auth password for `/metrics`; empty disables auth |

## Metrics

All per-device metrics carry a `device_id` label: the device's friendly ID as
shown in the admin UI. Telemetry a device has not reported yet is omitted
rather than exported as zero, so a freshly registered device does not look like
a flat battery.

| Metric                              | Type    | Meaning                                                         |
|-------------------------------------|---------|-----------------------------------------------------------------|
| `trmnl_battery_voltage_volts`       | gauge   | Battery voltage from the last display poll                      |
| `trmnl_battery_charging`            | gauge   | `1` while the device reports charging / USB connected, else `0` |
| `trmnl_wifi_rssi_dbm`               | gauge   | WiFi signal strength from the last display poll                 |
| `trmnl_last_seen_timestamp_seconds` | gauge   | Unix timestamp of the last display poll                         |
| `trmnl_refresh_rate_seconds`        | gauge   | Refresh interval the server hands out to the device             |
| `trmnl_device_info`                 | gauge   | Always `1`; labels `name`, `model`, `firmware_version`          |
| `trmnl_devices`                     | gauge   | Number of registered devices                                    |
| `trmnl_build_info`                  | gauge   | Always `1`; label `version`                                     |
| `trmnl_metrics_scrape_errors_total` | counter | Scrapes that failed to read devices from the database           |

The standard Go runtime and process collectors (`go_*`, `process_*`) are
exported as well.

## Scrape configuration

The [stack template](../deploy/monitoring/templates/trmnl/) ships this as a
drop-in file; for an existing Prometheus:

```yaml
scrape_configs:
  - job_name: trmnl
    scrape_interval: 60s
    static_configs:
      - targets: ["192.168.1.10:8080"]
    basic_auth:
      username: scraper
      password: s3cret
```

Scraping faster than the devices poll adds no information: a device wakes up
only every `refresh_rate` seconds (15 minutes by default), and the values
between two polls are constant.

## Alerting on battery and silence

Battery voltage is noisy: the reading depends on what the device was doing when
it measured, so alert on a smoothed value rather than a single sample.

```yaml
groups:
  - name: trmnl
    rules:
      - alert: TrmnlBatteryLow
        expr: avg_over_time(trmnl_battery_voltage_volts[6h]) < 3.4
        for: 1h
        annotations:
          summary: "{{ $labels.device_id }} battery is low"

      - alert: TrmnlBatteryCritical
        expr: avg_over_time(trmnl_battery_voltage_volts[6h]) < 3.3
        annotations:
          summary: "{{ $labels.device_id }} battery is nearly empty"

      # The discharge curve is flat in the middle and drops steeply at the end,
      # so predict_linear alone fires late; pair it with an absolute threshold,
      # and ignore devices that are charging.
      - alert: TrmnlBatteryTrend
        expr: |
          (
            predict_linear(trmnl_battery_voltage_volts[7d], 7 * 86400) < 3.3
              and avg_over_time(trmnl_battery_voltage_volts[6h]) < 3.7
          )
          unless trmnl_battery_charging == 1
        for: 6h
        annotations:
          summary: "{{ $labels.device_id }} battery will run out within a week"

      - alert: TrmnlSilent
        expr: time() - trmnl_last_seen_timestamp_seconds > clamp_min(3 * trmnl_refresh_rate_seconds, 1800)
        for: 15m
        annotations:
          summary: "{{ $labels.device_id }} missed several polls"
```

Tune the trend alert after a few weeks of data: the thresholds depend on the
battery and on how often the device wakes up. Voltage rises again while
charging, which is why `TrmnlBatteryTrend` excludes devices that report
themselves as charging. It uses `unless` rather than `and ... == 0` because
not every firmware sends a charging header: with `and`, the alert would never
fire for a device that omits it.

`TrmnlSilent` floors the window at 30 minutes. A device reports its own
refresh rate, and the server stores what it reports, so the value can be small
enough that three intervals are shorter than your scrape interval, which would
make the alert fire permanently.

Telemetry recorded before you enabled scraping stays in the trmnld database and
is visible in the admin UI; it is not backfilled into Prometheus.
