#!/usr/bin/env bash
# Download telemetry CSV files from the GitHub release into data/
set -euo pipefail

RELEASE_TAG="test-data-v1"
DEST_DIR="data"

mkdir -p "$DEST_DIR"
echo "Downloading test data from release '$RELEASE_TAG' into $DEST_DIR/ ..."
gh release download "$RELEASE_TAG" --dir "$DEST_DIR" --pattern "*.csv" --clobber
echo "Done. $(ls "$DEST_DIR"/*.csv 2>/dev/null | wc -l | tr -d ' ') CSV files in $DEST_DIR/"
