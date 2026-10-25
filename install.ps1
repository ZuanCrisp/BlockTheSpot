param(
  [switch] $Restore,
  [switch] $UpdateSpotify,
  [switch] $UninstallSpotifyStoreEdition
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repoRoot = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($repoRoot) -or -not (Test-Path -LiteralPath (Join-Path $repoRoot 'installer-cli\build.ps1'))) {
  $currentRoot = (Get-Location).Path
  if (Test-Path -LiteralPath (Join-Path $currentRoot 'installer-cli\build.ps1')) {
    $repoRoot = $currentRoot
  }
}

$downloadedSource = $false
if ([string]::IsNullOrWhiteSpace($repoRoot) -or -not (Test-Path -LiteralPath (Join-Path $repoRoot 'installer-cli\build.ps1'))) {
  $sourceRoot = Join-Path $env:TEMP "BlockTheSpot-source-$PID"
  $archivePath = Join-Path $env:TEMP "BlockTheSpot-source-$PID.zip"
  try {
    Write-Host 'Downloading BlockTheSpot CLI source from GitHub...'
    $archiveUri = 'https://codeload.github.com/ZuanCrisp/BlockTheSpot/zip/refs/heads/master?cache=' + [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    Invoke-WebRequest -UseBasicParsing -Uri $archiveUri -OutFile $archivePath
    Expand-Archive -LiteralPath $archivePath -DestinationPath $sourceRoot -Force
    $repoRoot = Join-Path $sourceRoot 'BlockTheSpot-master'
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot 'installer-cli\build.ps1'))) {
      throw 'The downloaded GitHub source does not contain the CLI installer.'
    }
    $downloadedSource = $true
  }
  catch {
    Remove-Item -LiteralPath $archivePath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $sourceRoot -Recurse -Force -ErrorAction SilentlyContinue
    throw
  }
  finally {
    Remove-Item -LiteralPath $archivePath -Force -ErrorAction SilentlyContinue
  }
}

$cliPath = Join-Path $repoRoot 'installer-cli\BlockTheSpotInstallerCLI.exe'
$exitCode = 1
try {
  if (-not (Test-Path -LiteralPath $cliPath)) {
    $goVersion = '1.24.5'
    $archiveName = "go$goVersion.windows-amd64.zip"
    $archiveHash = '658f432689106d4e0a401a2ebb522b1213f497bc8357142fe8def18d79f02957'
    $toolRoot = Join-Path $env:TEMP "BlockTheSpot-Go-$goVersion"
    $goExe = Join-Path $toolRoot 'go\bin\go.exe'
    $runtimeSource = Join-Path $toolRoot 'go\src\runtime\runtime.go'

    if (-not (Test-Path -LiteralPath $goExe) -or -not (Test-Path -LiteralPath $runtimeSource)) {
      $toolArchive = Join-Path $env:TEMP $archiveName
      if (-not (Test-Path -LiteralPath $toolArchive)) {
        Write-Host "Downloading portable Go $goVersion from go.dev..."
        Invoke-WebRequest -UseBasicParsing -Uri "https://go.dev/dl/$archiveName" -OutFile $toolArchive
      }

      $actualHash = (Get-FileHash -LiteralPath $toolArchive -Algorithm SHA256).Hash.ToLowerInvariant()
      if ($actualHash -ne $archiveHash) {
        Remove-Item -LiteralPath $toolArchive -Force
        throw 'Go archive checksum did not match the official checksum.'
      }

      New-Item -ItemType Directory -Path $toolRoot -Force | Out-Null
      Expand-Archive -LiteralPath $toolArchive -DestinationPath $toolRoot -Force
    }

    if (-not (Test-Path -LiteralPath $goExe) -or -not (Test-Path -LiteralPath $runtimeSource)) {
      throw 'Portable Go was not extracted correctly.'
    }

    Write-Host 'Building the console installer...'
    Push-Location (Join-Path $repoRoot 'installer-cli')
    try {
      & $goExe build -trimpath -ldflags='-s -w' -o $cliPath .
      if ($LASTEXITCODE -ne 0) {
        throw "CLI build failed with exit code $LASTEXITCODE."
      }
    }
    finally {
      Pop-Location
    }
    if (-not (Test-Path -LiteralPath $cliPath)) {
      throw 'The CLI build did not produce BlockTheSpotInstallerCLI.exe.'
    }
  }

  $cliArgs = @()
  if ($Restore) { $cliArgs += '--restore' }
  if ($UpdateSpotify) { $cliArgs += '--reinstall' }
  if ($UninstallSpotifyStoreEdition) { $cliArgs += '--remove-store-edition' }
  & $cliPath @cliArgs
  $exitCode = $LASTEXITCODE
}
finally {
  if ($downloadedSource) {
    Remove-Item -LiteralPath $sourceRoot -Recurse -Force -ErrorAction SilentlyContinue
  }
}

exit $exitCode
