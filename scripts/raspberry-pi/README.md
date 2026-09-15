# Raspberry Pi Installer

This script installs `trmnld` as a systemd service on a Raspberry Pi (or any other Debian-based Linux system).

Supported CPUs: ARMv7 (32-bit, e.g. Raspberry Pi 2/3/4 with a 32-bit OS), arm64 and amd64. ARMv6 boards (Raspberry Pi Zero/1) are not supported.

## Quick start

```bash
curl -fsSL -o install.sh \
  https://raw.githubusercontent.com/gesellix/go-trmnl/main/scripts/raspberry-pi/install.sh
sudo bash install.sh
```

This installs the latest release. Customize the installation with environment variables:

```bash
sudo \
  TRMNL_BASE_URL=http://trmnl.local:8080 \
  TRMNL_ADMIN_PASSWORD=change-me \
  bash install.sh
```

To install a specific release, pass its version:

```bash
sudo bash install.sh v0.4.0
```

## Updating

Re-run the installer, exactly as for a fresh installation:

```bash
curl -fsSL -o install.sh \
  https://raw.githubusercontent.com/gesellix/go-trmnl/main/scripts/raspberry-pi/install.sh
sudo bash install.sh            # latest release
sudo bash install.sh v0.4.0     # or a specific release
```

On an existing installation the script:

- downloads and verifies the new binary, keeping the previous one as `/usr/local/bin/trmnld.old`,
- rewrites the systemd unit and restarts the service,
- **keeps** your data directory (database, uploads, `secret.key`) and all existing values in `/etc/trmnld/trmnld.env`.

Environment variables passed to the installer only fill in keys that are missing from the env file. To change a setting such as `TRMNL_BASE_URL` or the admin password, edit `/etc/trmnld/trmnld.env` and restart the service:

```bash
sudo nano /etc/trmnld/trmnld.env
sudo systemctl restart trmnld
```

To roll back after a failed update, restore the previous binary:

```bash
sudo mv /usr/local/bin/trmnld.old /usr/local/bin/trmnld
sudo systemctl restart trmnld
```

Downgrading to an older release with a database that a newer release already migrated is not guaranteed to work, so back up the data directory before updating.

## Details

The script performs the following steps:

1. Resolves the version to install (the latest release unless one is given).
2. Switches to the installer of that release if it is newer than the running one.
3. Creates a `trmnl` system user and group.
4. Downloads the matching binary from GitHub Releases and verifies its SHA-256 checksum.
5. Adds missing settings to `/etc/trmnld/trmnld.env` (mode `0640`, owned by `root:trmnl`).
6. Installs the systemd unit `/etc/systemd/system/trmnld.service`.
7. Starts the service and checks that it responds on `/healthz`.

### Environment variables

| Variable               | Default                        | Description                                         |
|------------------------|--------------------------------|-----------------------------------------------------|
| `VERSION`              | `latest`                       | Release to install (same as the first argument)     |
| `TRMNL_LISTEN`         | `:8080`                        | HTTP listen address                                 |
| `TRMNL_BASE_URL`       | `http://<hostname>.local:8080` | Public URL the display uses to reach the server     |
| `TRMNL_DATA_DIR`       | `/var/lib/trmnld`              | Directory for the database, images and `secret.key` |
| `TRMNL_ADMIN_USER`     | `admin`                        | Admin username                                      |
| `TRMNL_ADMIN_PASSWORD` | (empty)                        | Admin password; empty disables admin authentication |

Further `trmnld` settings (for example `TRMNL_SECRET_KEY`) can be added to `/etc/trmnld/trmnld.env` by hand; see the [configuration reference](../../docs/GETTING-STARTED.md#configuration).

### Backups

Calendar credentials are encrypted with a key that `trmnld` generates on first start at `/var/lib/trmnld/secret.key` (unless `TRMNL_SECRET_KEY` is set). Back it up together with the database: without it, stored credentials must be entered again.

### Logs

```bash
journalctl -u trmnld -e --no-pager
```
