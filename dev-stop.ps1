$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$pidPath = Join-Path $repoRoot ".tmp\dev-local.pid"

if (!(Test-Path $pidPath)) {
  Write-Host "[dev] no recorded local development process."
  exit 0
}

$devPID = 0
if (!( [int]::TryParse((Get-Content $pidPath | Select-Object -First 1), [ref]$devPID) )) {
  Remove-Item -Force $pidPath
  throw "[dev] invalid PID file was removed."
}

$process = Get-Process -Id $devPID -ErrorAction SilentlyContinue
if ($process) {
  & taskkill.exe /PID $devPID /T /F | Out-Null
}

Remove-Item -Force $pidPath -ErrorAction SilentlyContinue
Write-Host "[dev] local Next.js and Go processes stopped. PostgreSQL is still running in Docker."
