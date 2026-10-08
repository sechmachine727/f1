#!/bin/sh
# Serve the built dashboard and run the telemetry bridge in the foreground.
# Any args (e.g. --no-agents, --replay FILE) pass straight through to the bridge.
set -e

python -m http.server "${WEB_PORT:-8081}" --bind 0.0.0.0 --directory /app/race_engineer_hub/dist &
exec python -m telemetry_server "$@"
