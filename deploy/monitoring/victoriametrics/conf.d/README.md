# Scrape jobs

One file per target, each holding a list of Prometheus `scrape_config` entries:

```yaml
- job_name: my-service
  scrape_interval: 60s
  static_configs:
    - targets: ["192.168.1.20:9100"]
```

VictoriaMetrics rereads this directory once a minute, so a new file needs no
restart. `%{ENV_VAR}` is substituted from the environment of the
victoriametrics container, which is how the go-trmnl template keeps its
scrape credentials in `.env`.

Check what is being scraped at <http://localhost:8428/targets>.
