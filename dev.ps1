param(
  [ValidateRange(1024, 65535)]
  [int]$PostgresPort = 55432
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$pidPath = Join-Path $repoRoot ".tmp\dev-local.pid"

function Get-ProjectSetting {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Name,
    [Parameter(Mandatory = $true)]
    [string]$Fallback
  )

  $processValue = [Environment]::GetEnvironmentVariable($Name, "Process")
  if (-not [string]::IsNullOrWhiteSpace($processValue)) {
    return $processValue
  }

  $envPath = Join-Path $repoRoot ".env"
  if (Test-Path $envPath) {
    foreach ($line in Get-Content $envPath) {
      if ($line -match "^\s*$([regex]::Escape($Name))\s*=\s*(.*)\s*$") {
        $value = $Matches[1].Trim().Trim('"').Trim("'")
        if (-not [string]::IsNullOrWhiteSpace($value)) {
          return $value
        }
      }
    }
  }

  return $Fallback
}

function Assert-PortAvailable {
  param([int]$Port)

  $listener = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
  if ($listener) {
    throw "[dev] port $Port is already in use. Stop the existing local process and run .\dev.ps1 again."
  }
}

Push-Location $repoRoot
try {
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $pidPath) | Out-Null
  Set-Content -Encoding ASCII -Path $pidPath -Value $PID

  docker version --format '{{.Server.Version}}' | Out-Null

  $env:POSTGRES_PUBLIC_PORT = [string]$PostgresPort
  Write-Host "[dev] stopping Docker application services; PostgreSQL data is preserved..."
  docker compose stop ai-pai admin api | Out-Host
  docker compose up -d postgres | Out-Host

  $postgresContainer = (docker compose ps -q postgres | Select-Object -First 1).Trim()
  if (-not $postgresContainer) {
    throw "[dev] PostgreSQL container was not created."
  }

  $postgresReady = $false
  for ($attempt = 0; $attempt -lt 30; $attempt++) {
    $postgresState = (docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' $postgresContainer).Trim()
    if ($postgresState -eq "healthy") {
      $postgresReady = $true
      break
    }
    Start-Sleep -Seconds 1
  }
  if (-not $postgresReady) {
    throw "[dev] PostgreSQL did not become healthy. Check: docker compose logs postgres"
  }

  $published = (docker compose port postgres 5432 | Select-Object -First 1).Trim()
  $portMatch = [regex]::Match($published, ':(\d+)$')
  if (-not $portMatch.Success) {
    throw "[dev] could not determine the PostgreSQL host port."
  }

  Assert-PortAvailable -Port 3000
  Assert-PortAvailable -Port 3001
  Assert-PortAvailable -Port 3002

  $env:GO_BACKEND_URL = "http://127.0.0.1:3001"
  $env:ADMIN_INTERNAL_URL = "http://127.0.0.1:3002"
  $env:APP_PUBLIC_ORIGIN = "http://127.0.0.1:3000"
  $env:ADMIN_PUBLIC_ORIGIN = "http://127.0.0.1:3000"
  $env:AUTH_ACTION_URLS_IN_RESPONSE = "true"
  $env:ADMIN_COOKIE_SECURE = "false"
  $env:DB_DRIVER = "postgres"
  $env:DB_HOST = "127.0.0.1"
  $env:DB_PORT = $portMatch.Groups[1].Value
  $env:DB_NAME = Get-ProjectSetting -Name "DB_NAME" -Fallback "ai_pai"
  $env:DB_USER = Get-ProjectSetting -Name "DB_USER" -Fallback "ai_pai"
  $env:DB_PASSWORD = Get-ProjectSetting -Name "DB_PASSWORD" -Fallback "ai_pai_change_me"
  $env:DB_SSLMODE = Get-ProjectSetting -Name "DB_SSLMODE" -Fallback "disable"

  Write-Host "[dev] PostgreSQL: Docker on 127.0.0.1:$($env:DB_PORT)"
  Write-Host "[dev] customer app: http://127.0.0.1:3000"
  Write-Host "[dev] admin app:    http://127.0.0.1:3000/sys-admins"
  Write-Host "[dev] Next.js and Go now use hot reload. Press Ctrl+C to stop local services."
  npm run dev
}
finally {
  if (Test-Path $pidPath) {
    $recordedPID = Get-Content $pidPath -ErrorAction SilentlyContinue
    if ([string]$recordedPID -eq [string]$PID) {
      Remove-Item -Force $pidPath
    }
  }
  Pop-Location
}
