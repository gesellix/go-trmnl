# Raspberry Pi Installer

Dieses Skript installiert `trmnld` als Systemd-Service auf einem Raspberry Pi (oder einem anderen Debian-basierten Linux-System).

## Quick Start

```bash
curl -fsSL -o install.sh \
  https://raw.githubusercontent.com/gesellix/go-trmnl/main/scripts/raspberry-pi/install.sh
sudo bash install.sh
```

Passen Sie die Installation über Umgebungsvariablen an:

```bash
sudo \
  TRMNL_BASE_URL=http://trmnl.local:8080 \
  bash install.sh
```

## Details

Das Skript führt folgende Schritte aus:
1. Erstellt einen `trmnl` System-Benutzer und -Gruppe.
2. Lädt das passende Binary von GitHub Releases herunter.
3. Konfiguriert die Umgebungsvariablen in `/etc/trmnld/trmnld.env`.
4. Installiert eine Systemd-Unit `/etc/systemd/system/trmnld.service`.
5. Startet den Service und prüft die Erreichbarkeit der Admin-Oberfläche.

### Umgebungsvariablen

| Variable               | Standard                       | Beschreibung                    |
|------------------------|--------------------------------|---------------------------------|
| `TRMNL_LISTEN`         | `:8080`                        | HTTP Listen-Adresse             |
| `TRMNL_BASE_URL`       | `http://<hostname>.local:8080` | Öffentliche URL für das Display |
| `TRMNL_DATA_DIR`       | `/var/lib/trmnld`              | Verzeichnis für DB und Bilder   |
| `TRMNL_ADMIN_USER`     | `admin`                        | Admin-Benutzername              |
| `TRMNL_ADMIN_PASSWORD` | (leer)                         | Admin-Passwort                  |
