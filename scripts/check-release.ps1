$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$required = @(".next/standalone/server.js", "release/ai-pai.exe")
$missing = $required | Where-Object { !(Test-Path (Join-Path $repoRoot $_)) }
if ($missing.Count -gt 0) {
  throw "Release is incomplete: $($missing -join ', ')"
}
Write-Host "Release check passed."
