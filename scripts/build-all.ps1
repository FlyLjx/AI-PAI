param()

$ErrorActionPreference = "Stop"

Push-Location (Split-Path -Parent $PSScriptRoot)
try {
  npm run build:web
  npm run build:admin
}
finally {
  Pop-Location
}
& (Join-Path $PSScriptRoot "build-go.ps1")
