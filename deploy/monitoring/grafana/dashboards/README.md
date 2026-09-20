# Dashboards

Grafana loads every `*.json` in this directory and reloads it within 30
seconds of a change. Dashboards are read-only in the UI: edit the file, or use
**Export → Save to file** after "Edit" to write a new version here.

`./add-template.sh trmnl` installs the go-trmnl dashboard.
