#!/usr/bin/env bash
set -euo pipefail

# Installs a template from templates/<name>/ into the running stack:
#
#   scrape/*.yml      -> victoriametrics/conf.d/
#   rules/*.yml       -> vmalert/rules/
#   dashboards/*.json -> grafana/dashboards/
#
# Existing files are never overwritten silently; pass -f to replace them.
# Usage: ./add-template.sh [-f] <name>

force=0
while getopts ":f" opt; do
  case "$opt" in
    f) force=1 ;;
    *) echo "usage: $0 [-f] <template>" >&2; exit 2 ;;
  esac
done
shift $((OPTIND - 1))

name="${1:-}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
src="$here/templates/$name"

if [ -z "$name" ] || [ ! -d "$src" ]; then
  echo "usage: $0 [-f] <template>" >&2
  echo "available:" >&2
  for d in "$here"/templates/*/; do [ -d "$d" ] && echo "  $(basename "$d")" >&2; done
  exit 2
fi

install_dir() {
  local from="$src/$1" to="$here/$2" pattern="$3" copied=0
  [ -d "$from" ] || return 0
  mkdir -p "$to"
  shopt -s nullglob
  for f in "$from"/$pattern; do
    local target="$to/$(basename "$f")"
    if [ -e "$target" ] && [ "$force" -eq 0 ]; then
      echo "  skipped $2/$(basename "$f") (exists, use -f to replace)"
    else
      cp "$f" "$target"
      echo "  installed $2/$(basename "$f")"
      copied=$((copied + 1))
    fi
  done
  shopt -u nullglob
  return 0
}

echo "Installing template '$name':"
install_dir scrape     victoriametrics/conf.d '*.yml'
install_dir rules      vmalert/rules          '*.yml'
install_dir dashboards grafana/dashboards     '*.json'

if [ -f "$src/README.md" ]; then
  echo
  echo "Next steps: templates/$name/README.md"
fi

cat <<'MSG'

Then apply the changes:

  docker compose up -d            # picks up scrape.env changes
  docker compose restart vmalert  # rules are loaded at start

Scrape jobs and dashboards are picked up automatically within a minute.
MSG
