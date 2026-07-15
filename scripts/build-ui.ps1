$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
  npm run build:web
  npm run build:admin
}
finally {
  Pop-Location
}
