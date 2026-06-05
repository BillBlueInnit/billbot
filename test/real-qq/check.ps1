param(
  [int]$Port = 2006
)

$ErrorActionPreference = "Stop"

function Show-Json($label, $value) {
  Write-Host ""
  Write-Host "== $label =="
  $value | ConvertTo-Json -Depth 8
}

$base = "http://127.0.0.1:$Port"
Show-Json "BillBot health" (Invoke-RestMethod "$base/api/health" -TimeoutSec 5)
Show-Json "NapCat connector" (Invoke-RestMethod "$base/api/connectors/status" -TimeoutSec 5)
Show-Json "Bridge status" (Invoke-RestMethod "$base/api/bridge/status" -TimeoutSec 5)
Show-Json "Diagnostics" (Invoke-RestMethod "$base/api/diagnostics" -TimeoutSec 30)

Write-Host ""
Write-Host "If connector.connected is true and diagnostics.hermes.command_found/chat_ok are true, send this in your QQ group:"
Write-Host "  @your-bot /ping"
