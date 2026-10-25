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
    Invoke-WebRequest -UseBasicParsing -Uri 'https://codeload.github.com/ZuanCrisp/BlockTheSpot/zip/refs/heads/master' -OutFile $archivePath
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
$buildScript = Join-Path $repoRoot 'installer-cli\build.ps1'
$exitCode = 1
try {
  if (-not (Test-Path -LiteralPath $cliPath)) {
    & $buildScript
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
