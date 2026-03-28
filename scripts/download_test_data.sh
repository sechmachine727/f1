#!/usr/bin/env bash
# Download telemetry .f1bin files from the GitHub release into data/
set -euo pipefail

RELEASE_TAG="test-data-v2"
DEST_DIR="data"

mkdir -p "$DEST_DIR"
echo "Downloading test data from release '$RELEASE_TAG' into $DEST_DIR/ ..."
gh release download "$RELEASE_TAG" --dir "$DEST_DIR" --pattern "*.f1bin" --clobber
echo "Done. $(ls "$DEST_DIR"/*.f1bin 2>/dev/null | wc -l | tr -d ' ') .f1bin files in $DEST_DIR/"
