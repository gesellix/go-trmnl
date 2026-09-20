# Monitoring stack

A small metrics stack for a Raspberry Pi or NAS: scraping, storage, alerting
and dashboards in five containers. Nothing in it is go-trmnl specific; the
go-trmnl bits are a template you install on top (see
[templates/trmnl](templates/trmnl/README.md)).

| Container | Image | Job |
|-----------|-------|-----|
| victoriametrics | `victoriametrics/victoria-metrics` | Scrapes the targets and stores the samples |
| vmalert | `victoriametrics/vmalert` | Evaluates the alert rules |
| alertmanager | `prom/alertmanager` | Groups, silences and routes alerts |
| ntfy-alertmanager | `xenrox/ntfy-alertmanager` | Turns alerts into readable ntfy push messages |
| grafana | `grafana/grafana-oss` | Dashboards |

VictoriaMetrics does the scraping itself, so there is no separate agent. It
speaks PromQL and the Prometheus scrape config format: anything written for
Prometheus works here, and swapping it for Prometheus later means reusing the
same rule and scrape files.

## Requirements

Docker with the Compose plugin, on a 64-bit OS. Pi OS 64-bit on a Pi 4 or 5,
or any x86 NAS, is plenty; a 32-bit install will not work because the
ntfy-alertmanager image has no `linux/arm/v7` build (drop that one service and
the rest runs on 32-bit).

```sh
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"   # log out and back in
```

## Quick start

```sh
git clone https://github.com/gesellix/go-trmnl
cd go-trmnl/deploy/monitoring

cp .env.example .env
$EDITOR .env                       # at least GRAFANA_PASSWORD and TZ

$EDITOR alertmanager/ntfy-alertmanager.scfg   # set your ntfy topic

docker compose up -d
```

Grafana is then at `http://<host>:3000`. VictoriaMetrics (`:8428`), vmalert
(`:8880`) and Alertmanager (`:9093`) bind to localhost only by default; set
`VM_ADDR=0.0.0.0` in `.env` to reach their web UIs from the LAN.

Add go-trmnl:

```sh
./add-template.sh trmnl
```

and follow [templates/trmnl/README.md](templates/trmnl/README.md).

### Notifications

The stack pushes to [ntfy](https://ntfy.sh): install the app, subscribe to a
topic, and put that topic in `alertmanager/ntfy-alertmanager.scfg`. On the
public server the topic name is the only protection, so pick something long
and random. Point `server` at your own ntfy instance if you run one.

To use something else (email, Gotify, a webhook), replace the `ntfy` receiver
in `alertmanager/alertmanager.yml` with any
[Alertmanager receiver](https://prometheus.io/docs/alerting/latest/configuration/#receiver)
and drop the `ntfy-alertmanager` service from `compose.yaml`.

## Layout

```
compose.yaml                     the five services
.env                             ports, passwords, retention (from .env.example)
scrape.env                       credentials for scrape jobs (from scrape.env.example)
victoriametrics/scrape.yml       base scrape config, self-monitoring
victoriametrics/conf.d/*.yml     one file per target        <- templates land here
vmalert/rules/*.yml              alert rules                <- templates land here
alertmanager/alertmanager.yml    routing, grouping, inhibition
alertmanager/ntfy-alertmanager.scfg  ntfy topic and priorities
grafana/provisioning/            datasource and dashboard providers
grafana/dashboards/*.json        dashboards                 <- templates land here
templates/<name>/                installable bundles
```

Scrape jobs and dashboards are picked up automatically (within a minute and 30
seconds respectively). New alert rules need `docker compose restart vmalert`.

Credentials for scrape targets live in `scrape.env`, which is handed to the
VictoriaMetrics container only, and are referenced in scrape configs as
`%{VAR_NAME}`. Both `.env` and `scrape.env` are gitignored, as are the files
`add-template.sh` copies into place.

## Footprint

Measured with one go-trmnl server and one device, idle, on a 64-bit host:

| | Memory | CPU (idle) | Image on disk | Download (arm64) |
|---|---|---|---|---|
| victoriametrics | 120 MB | 0.2% | 54 MB | 18 MB |
| vmalert | 16 MB | 0.3% | 53 MB | 18 MB |
| alertmanager | 17 MB | 0.1% | 117 MB | 38 MB |
| ntfy-alertmanager | 6 MB | 0.0% | 42 MB | 12 MB |
| grafana | 146 MB | 0.6% | 1.4 GB | 328 MB |
| **total** | **~305 MB** | **~1%** | **~1.7 GB** | **~415 MB** |

Grafana is two thirds of the download and half the memory. Leaving it out and
using the built-in VictoriaMetrics UI (`http://<host>:8428/vmui`) turns this
into a ~160 MB stack.

**Data growth.** The stack stores roughly 760k samples a day: the three
self-monitoring jobs contribute ~2300 series at a 5 minute interval, go-trmnl
~58 series (of which ~9 per device, the rest Go runtime metrics) at 60
seconds. VictoriaMetrics compresses slowly changing gauges to well under a
byte per sample, which puts a stack like this at a few hundred megabytes per
year. Each additional device adds ~9 series, about 13k samples a day, which is
noise in comparison.

Defaults that keep it that way, all in `.env`:

- `VM_RETENTION=12` — samples older than 12 months are deleted. Months, or
  `1y`, `3y`, `100y` if you never want to lose anything.
- `VM_MEMORY_PERCENT=30` — VictoriaMetrics caps its caches at 30% of system
  memory instead of the default 60%.
- Container logs rotate at 10 MB with three files kept, per service.

The stack alerts on its own disk usage (`DiskFillingUp`, below 10% free). To
see what it actually uses:

```sh
docker system df -v | grep monitoring        # volume sizes
curl -s localhost:8428/api/v1/status/tsdb    # series counts
```

Dropping the Go runtime metrics of a scrape target cuts most of its series;
`templates/trmnl/scrape/trmnl.yml` has the three-line `metric_relabel_configs`
for it, commented out.

## Operating it

```sh
docker compose ps                     # what is running
docker compose logs -f vmalert        # why a rule is not firing
docker compose pull && docker compose up -d   # update images
docker compose down                   # stop (volumes survive)
docker compose down -v                # stop and delete all metrics
```

Useful pages: `http://<host>:8428/targets` (is the scrape working),
`http://<host>:8428/vmui` (ad-hoc queries), `http://<host>:8880` (rule state),
`http://<host>:9093` (firing alerts, silences).

**Backups.** Grafana dashboards and all configuration are files in this
directory, so a copy of it is the backup. The metrics themselves live in the
`monitoring_vm-data` volume; for a home setup, losing them means losing
history, not configuration. If you do want them:

```sh
docker compose exec victoriametrics wget -qO- 'http://127.0.0.1:8428/snapshot/create'
```

then copy the named snapshot out of the volume.

**Security.** Everything here assumes a trusted LAN: Grafana is the only
service exposed by default, and it has a password. Do not port-forward any of
these to the internet. If you need remote access, put a VPN (WireGuard,
Tailscale) in front rather than opening ports.
