#!/bin/sh
# Build the dashboard and refresh the embedded copy. Run before a plain go build.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root/race_engineer_hub"
[ -d node_modules ] || npm ci
npm run build
rm -rf "$root/server/internal/web/dist"
mkdir -p "$root/server/internal/web/dist"
cp -R dist/. "$root/server/internal/web/dist/"
echo "embedded dashboard from race_engineer_hub/dist"
