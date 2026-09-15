---
layout: default
title: go-trmnl
---

# Your TRMNL, your server.

A self-hosted server for the [TRMNL](https://usetrmnl.com) e-ink display,
written in Go. It replaces TRMNL's hosted backend, so a device on your network
is provisioned, fetches screens, and reports telemetry without third-party
servers in the middle. Your data and calendars stay on your own machine.
{: .lead}

## What it does

- **Screens from plugins:** clock, weather, a family calendar, days left in the
  year, quotes, and your own images, rendered to 800×480 black and white.
- **Playlists** that each device cycles through.
- **Family calendar** merged from Google and Apple iCloud/CalDAV accounts, with
  credentials encrypted at rest.
- **Web admin UI** for devices, screens, playlists, calendars, logs, and
  settings, optionally over HTTPS.
- **One static binary** or a small container, down to a Raspberry Pi.

## Get started

On a Raspberry Pi or another Debian-based system:

```sh
curl -fsSL -o install.sh \
  https://raw.githubusercontent.com/gesellix/go-trmnl/main/scripts/raspberry-pi/install.sh
sudo bash install.sh
```

Or with Docker:

```sh
docker run -d -p 8080:8080 -v trmnl-data:/data \
  -e TRMNL_BASE_URL=http://<your-lan-ip>:8080 \
  -e TRMNL_ADMIN_PASSWORD=changeme \
  ghcr.io/gesellix/go-trmnl:latest
```

Then open `http://<your-lan-ip>:8080/admin` and point your TRMNL device at the
server. The [getting started guide](https://github.com/gesellix/go-trmnl/blob/main/docs/GETTING-STARTED.md)
walks through every step.

## Open source

go-trmnl is MIT-licensed and developed on
[GitHub](https://github.com/gesellix/go-trmnl). It is an independent project,
not affiliated with TRMNL. Questions and ideas are welcome as
[issues](https://github.com/gesellix/go-trmnl/issues).
