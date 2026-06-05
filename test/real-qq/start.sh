#!/usr/bin/env sh
set -eu

PORT="${PORT:-2006}"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
CONFIG="${CONFIG:-$SCRIPT_DIR/config.real-qq.yaml}"
EXE="$REPO_DIR/bin/billbot-real-qq"

mkdir -p "$REPO_DIR/bin"
cd "$REPO_DIR"
go build -o "$EXE" ./cmd/billbot

echo "Starting BillBot real QQ test..."
echo "Dashboard: http://127.0.0.1:$PORT"
echo "Config: $CONFIG"
echo "Press Ctrl+C to stop."
exec "$EXE" --config "$CONFIG" --port "$PORT"
