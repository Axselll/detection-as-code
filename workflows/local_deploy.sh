#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."

if [ -f "$ROOT_DIR/.env" ]; then
  export $(grep -v -E '^\s*#|^\s*$' "$ROOT_DIR/.env" | xargs)
else
  echo ".env file not found at $ROOT_DIR/.env!"
  exit 1
fi

cd "$ROOT_DIR"


go build -o build main.go


./build

TARBALL_PATH="$ROOT_DIR/app/detections_app.tar.gz"

if [ ! -f "$TARBALL_PATH" ]; then
  echo "Tarball not found at $TARBALL_PATH!"
  exit 1
fi

if ! file "$TARBALL_PATH" | grep -q 'gzip compressed data'; then
  echo "Warning: Tarball doesn't look like a valid gzip archive."
fi


# curl -k -H "Authorization: Bearer $SPLUNK_TOKEN" https://$SPLUNK_HOST:8089/services/apps/local -F "name=detections_app" -F "app=@$TARBALL_PATH"

# curl -k -X POST -H "Authorization: Bearer $SPLUNK_TOKEN" \
#      -F "name=detections_app" \
#      -F "filename=@${TARBALL_PATH}" \
#      https://$SPLUNK_HOST:8089/services/apps/local
