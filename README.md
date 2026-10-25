# BlockTheSpot CLI

Windows x64 console installer. It reads the latest BlockTheSpot release from
[Nuzair46/BlockTheSpot](https://github.com/Nuzair46/BlockTheSpot) and filters the
Spotify version list from [Loadspot](https://loadspot.pages.dev/versions.json)
to versions compatible with that release. It skips unsupported versions rather
than patching them blindly.

## Run from PowerShell

From a local clone or downloaded source folder:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```

The first run builds a console program from `installer-cli` source. If Go is not
installed, the build script downloads a pinned Go toolchain to the temp folder,
checks its SHA-256, and does not install it system-wide. The installer then shows
the compatible Spotify version choices in the terminal.

To restore Spotify, run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Restore
```

The batch launchers call the same CLI:

- `BlockTheSpot.bat` opens the install/update menu.
- `uninstall.bat` opens the restore action directly.

For a remote PowerShell launch, the script downloads this repository's source
from GitHub, builds the CLI, then runs it:

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-Expression "& { $(Invoke-WebRequest -UseBasicParsing 'https://raw.githubusercontent.com/ZuanCrisp/BlockTheSpot/master/install.ps1') }"
```

The CLI asks before replacing Spotify with a selected compatible version or
removing a Microsoft Store edition. The selection is based on the patch release's
compatibility metadata, so it can offer an older Spotify build when the newest
one is unsupported. Keep using this CLI for updates; the old installer that
always downloaded Spotify's latest setup has been removed.

## Build manually

```powershell
cd installer-cli
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
.\BlockTheSpotInstallerCLI.exe
```

The CLI source adapts the installer flow from
[Nuzair46/BlockTheSpot-Installer](https://github.com/Nuzair46/BlockTheSpot-Installer)
under its MIT license. The web version list, release asset checks, version
compatibility checks, file backup and restore, and diagnostic reporting are kept
in the CLI implementation.
