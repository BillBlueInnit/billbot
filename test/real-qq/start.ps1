param(
  [int]$Port = 2006,
  [string]$Config = "$PSScriptRoot\config.real-qq.yaml"
)

$ErrorActionPreference = "Stop"
$repo = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$exe = Join-Path $repo "bin\billbot-real-qq.exe"
$go = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $go -and (Test-Path "D:\golang\go\bin\go.exe")) {
  $go = "D:\golang\go\bin\go.exe"
}
if (-not $go) {
  throw "Go was not found. Install Go or add go.exe to PATH."
}

New-Item -ItemType Directory -Force -Path (Split-Path $exe) | Out-Null
Push-Location $repo
try {
  & $go build -o $exe .\cmd\billbot
  Write-Host "Starting BillBot real QQ test..."
  Write-Host "Dashboard: http://127.0.0.1:$Port"
  Write-Host "Config: $Config"
  Write-Host "Press Ctrl+C to stop."
  & $exe --config $Config --port $Port
} finally {
  Pop-Location
}
