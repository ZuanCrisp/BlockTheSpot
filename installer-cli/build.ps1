$ErrorActionPreference = 'Stop'

$goVersion = '1.24.5'
$archiveName = "go$goVersion.windows-amd64.zip"
$archiveUrl = "https://go.dev/dl/$archiveName"
$archiveHash = '658f432689106d4e0a401a2ebb522b1213f497bc8357142fe8def18d79f02957'
$toolRoot = Join-Path $env:TEMP "BlockTheSpot-Go-$goVersion"
$goExe = Join-Path $toolRoot 'go\bin\go.exe'

if (-not (Test-Path -LiteralPath $goExe) -or -not (Test-Path -LiteralPath (Join-Path $toolRoot 'go\src\runtime\runtime.go'))) {
  $archivePath = Join-Path $env:TEMP $archiveName
  if (-not (Test-Path -LiteralPath $archivePath)) {
    Write-Host "Downloading portable Go $goVersion from go.dev..."
    Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath
  }

  $actualHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($actualHash -ne $archiveHash) {
    Remove-Item -LiteralPath $archivePath -Force
    throw 'Go archive checksum did not match the official checksum.'
  }

  New-Item -ItemType Directory -Path $toolRoot -Force | Out-Null
  Expand-Archive -LiteralPath $archivePath -DestinationPath $toolRoot -Force
}

$outputPath = Join-Path $PSScriptRoot 'BlockTheSpotInstallerCLI.exe'
Write-Host 'Building the console installer...'
Push-Location $PSScriptRoot
try {
  & $goExe build -trimpath -ldflags='-s -w' -o $outputPath .
  if ($LASTEXITCODE -ne 0) {
    throw "Go build failed with exit code $LASTEXITCODE."
  }
}
finally {
  Pop-Location
}

Write-Host "Built $outputPath"
