$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $repoRoot
try {
  docker compose up -d --build
  $published = (docker compose port ai-pai 3000 | Select-Object -First 1).Trim()
  $portMatch = [regex]::Match($published, ':(\d+)$')
  $publicPort = if ($portMatch.Success) { $portMatch.Groups[1].Value } else { "6985" }
  Write-Host "AI-PAI web: http://127.0.0.1:$publicPort"
  Write-Host "AI-PAI admin: http://127.0.0.1:$publicPort/sys-admins"
}
finally {
  Pop-Location
}
