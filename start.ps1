$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $repoRoot
try {
  docker compose up -d --build
  Write-Host "AI-PAI: http://127.0.0.1:6985"
}
finally {
  Pop-Location
}
