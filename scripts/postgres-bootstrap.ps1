$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$goServer = Join-Path $repoRoot "go-server"

Write-Host "[postgres] starting postgres container..."
docker compose up -d postgres

Write-Host "[postgres] waiting for postgres health..."
$ready = $false
for ($i = 0; $i -lt 60; $i++) {
  $status = docker inspect --format='{{.State.Health.Status}}' ai-pai-postgres 2>$null
  if ($status -eq "healthy") {
    $ready = $true
    break
  }
  Start-Sleep -Seconds 2
}
if (-not $ready) {
  throw "postgres container did not become healthy in time"
}

Push-Location $goServer
try {
  $env:DB_DRIVER = if ($env:DB_DRIVER) { $env:DB_DRIVER } else { "postgres" }
  $env:DB_HOST = if ($env:DB_HOST) { $env:DB_HOST } else { "127.0.0.1" }
  $env:DB_PORT = if ($env:DB_PORT) { $env:DB_PORT } else { "5432" }
  $env:DB_USER = if ($env:DB_USER) { $env:DB_USER } else { "ai_pai" }
  $env:DB_PASSWORD = if ($env:DB_PASSWORD) { $env:DB_PASSWORD } else { "ai_pai_change_me" }
  $env:DB_NAME = if ($env:DB_NAME) { $env:DB_NAME } else { "ai_pai" }
  $env:DB_SSLMODE = if ($env:DB_SSLMODE) { $env:DB_SSLMODE } else { "disable" }

  Write-Host "[postgres] migrating mysql business data into postgres..."
  go run ./cmd/pgmigrate

  Write-Host "[postgres] running postgres smoke checks..."
  go run ./cmd/pgsmoke
}
finally {
  Pop-Location
}

Write-Host "[postgres] starting app container..."
docker compose up -d ai-pai

Write-Host "[postgres] done"
