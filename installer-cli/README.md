# BlockTheSpot CLI installer

This console installer adapts the version selection and safe install flow from
[Nuzair46/BlockTheSpot-Installer](https://github.com/Nuzair46/BlockTheSpot-Installer).
It retrieves the latest BlockTheSpot release and Spotify version catalog over
HTTPS, then lists only Spotify versions compatible with that release.

Run `build.ps1` to build the Windows console executable. If Go is not installed,
the script downloads the pinned Go 1.24.5 Windows archive to the temp folder and
checks its SHA-256 before extracting it. It does not install Go system-wide.

Then run `BlockTheSpotInstallerCLI.exe` from PowerShell or Command Prompt. Use
`BlockTheSpotInstallerCLI.exe --restore` to open the restore flow directly.

The CLI requires Windows x64. It uses Nuzair46's published BlockTheSpot release
and version catalog. It will not patch a Spotify version that does not match the
selected release. Installation can replace Spotify with the selected compatible
version; the CLI asks before continuing.

The source and installer files adapted from Nuzair46's installer are covered by
the included MIT license.
