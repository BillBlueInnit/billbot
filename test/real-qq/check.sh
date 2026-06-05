#!/usr/bin/env sh
set -eu

PORT="${PORT:-2006}"
BASE="http://127.0.0.1:$PORT"

echo "== BillBot health =="
curl -fsS "$BASE/api/health"
echo
echo
echo "== NapCat connector =="
curl -fsS "$BASE/api/connectors/status"
echo
echo
echo "== Bridge status =="
curl -fsS "$BASE/api/bridge/status"
echo
echo
echo "== Diagnostics =="
curl -fsS "$BASE/api/diagnostics"
echo
echo
echo "If connector.connected is true and diagnostics.hermes.command_found/chat_ok are true, send this in your QQ group:"
echo "  @your-bot /ping"
