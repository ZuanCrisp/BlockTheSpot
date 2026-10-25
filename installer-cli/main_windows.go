//go:build windows

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	spotifyVersionsURL = "https://loadspot.pages.dev/versions.json"
)

type installOptions struct {
	ResetSettings       bool
	UpdateSpotify       bool
	LaunchSpotifyOnDone bool
	SpotifyVersion      spotifyInstallChoice
}

type installer struct {
	options     installOptions
	release     *patchRelease
	logf        func(format string, args ...any)
	setProgress func(value int)
	setStatus   func(status string)
}

var console = bufio.NewReader(os.Stdin)

func main() {
	forceReinstall := false
	allowStoreRemoval := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--restore":
			runUninstallFlow()
			return
		case "--reinstall":
			forceReinstall = true
		case "--remove-store-edition":
			allowStoreRemoval = true
		default:
			fmt.Printf("Unknown option: %s\nUsage: BlockTheSpotInstallerCLI.exe [--restore] [--reinstall] [--remove-store-edition]\n", arg)
			return
		}
	}
	fmt.Println("BlockTheSpot CLI installer")
	fmt.Println("Version options and compatible patch files are loaded from the web.")
	fmt.Println()

	for {
		fmt.Println("1) Install / Update")
		fmt.Println("2) Uninstall / Restore")
		fmt.Println("3) Exit")
		choice := prompt("Select an option [1-3]: ")
		fmt.Println()

		switch choice {
		case "1":
			if runInstallFlow(forceReinstall, allowStoreRemoval) {
				return
			}
		case "2":
			if runUninstallFlow() {
				return
			}
		case "3", "q", "Q":
			return
		default:
			fmt.Println("Enter 1, 2, or 3.")
			fmt.Println()
		}
	}
}

func runInstallFlow(forceDefault, allowStoreRemoval bool) bool {
	fmt.Println("Checking the current BlockTheSpot release and supported Spotify versions...")
	release, versions, recommended, err := fetchSpotifyInstallChoices()
	if err != nil {
		fmt.Printf("Could not load the web version list: %v\n\n", err)
		return false
	}
	if release == nil || len(versions) == 0 {
		fmt.Println("No compatible Spotify versions are available from the current patch release.")
		fmt.Println()
		return false
	}

	installed := "not installed"
	spotifyExe := filepath.Join(defaultSpotifyDir(), "Spotify.exe")
	if fileExists(spotifyExe) {
		if version, err := getSpotifyVersion(spotifyExe); err == nil {
			installed = baseSpotifyVersion(version)
		} else {
			installed = "unknown (" + err.Error() + ")"
		}
	}

	fmt.Printf("Patch release: %s\n", release.Tag)
	fmt.Printf("Required Spotify version: %s\n", release.Compatibility.Version)
	fmt.Printf("Currently installed: %s\n\n", installed)
	fmt.Println("Compatible Spotify versions from the web catalog:")
	for index, version := range versions {
		marker := ""
		if index == recommended {
			marker = " [recommended]"
		}
		if baseSpotifyVersion(installed) == version.BaseVersion {
			marker += " [already installed]"
		}
		date := ""
		if version.Date != "" {
			date = " - " + version.Date
		}
		fmt.Printf("  %d) %s%s%s\n", index+1, version.FullVersion, date, marker)
	}
	fmt.Println()

	selectedIndex := promptVersionIndex(len(versions), recommended)
	if selectedIndex < 0 {
		fmt.Println("Cancelled.")
		fmt.Println()
		return false
	}
	selected := versions[selectedIndex]

	force := askYesNo("Reinstall this Spotify version even if it already matches? [y/N]: ", forceDefault)
	launch := askYesNo("Launch Spotify after installation? [Y/n]: ", true)
	fmt.Println()
	fmt.Printf("Spotify will be set to version %s before patching.\n", selected.BaseVersion)
	fmt.Printf("Existing Spotify version: %s\n", installed)
	if !askYesNo("Continue with Install / Update? [y/N]: ", false) {
		fmt.Println("Cancelled.")
		fmt.Println()
		return false
	}

	storeInstalled, storeErr := isSpotifyStoreInstalled()
	if storeErr == nil && storeInstalled {
		fmt.Println("Microsoft Store Spotify was detected. It must be removed before installing the desktop edition.")
		if !allowStoreRemoval && !askYesNo("Allow the installer to remove the Microsoft Store edition? [y/N]: ", false) {
			fmt.Println("Cancelled. The Store edition was left installed.")
			fmt.Println()
			return false
		}
	}

	i := newCLIInstaller(release)
	i.options = installOptions{
		UpdateSpotify:       force,
		LaunchSpotifyOnDone: launch,
		SpotifyVersion:      selected,
	}
	if err := i.runInstall(); err != nil {
		fmt.Printf("\nInstall failed: %v\n", err)
		fmt.Println("Spotify has not been patched with a version that does not match the selected release.")
		fmt.Println()
		return false
	}
	fmt.Println("\nInstall completed.")
	fmt.Println()
	return launch
}

func runUninstallFlow() bool {
	fmt.Println("Uninstall / Restore removes BlockTheSpot files and restores Spotify's matching original DLL.")
	if !askYesNo("Continue with Uninstall / Restore? [y/N]: ", false) {
		fmt.Println("Cancelled.")
		fmt.Println()
		return false
	}
	i := newCLIInstaller(nil)
	i.options.LaunchSpotifyOnDone = askYesNo("Launch Spotify after restore? [Y/n]: ", true)
	if err := i.runUninstall(); err != nil {
		fmt.Printf("\nRestore failed: %v\n\n", err)
		return false
	}
	fmt.Println("\nSpotify was restored.")
	fmt.Println()
	return i.options.LaunchSpotifyOnDone
}

func newCLIInstaller(release *patchRelease) *installer {
	return &installer{
		release: release,
		logf: func(format string, args ...any) {
			fmt.Printf(format+"\n", args...)
		},
		setStatus: func(status string) {
			fmt.Printf("\n== %s ==\n", status)
		},
		setProgress: func(value int) {
			if value > 0 && value%10 == 0 {
				fmt.Printf("Progress: %d%%\n", value)
			}
		},
	}
}

func prompt(message string) string {
	fmt.Print(message)
	line, err := console.ReadString('\n')
	if errors.Is(err, io.EOF) {
		return "3"
	}
	return strings.TrimSpace(line)
}

func askYesNo(message string, defaultYes bool) bool {
	answer := strings.ToLower(prompt(message))
	if answer == "" {
		return defaultYes
	}
	return answer == "y" || answer == "yes"
}

func promptVersionIndex(count, recommended int) int {
	for {
		defaultValue := recommended + 1
		answer := prompt(fmt.Sprintf("Choose a Spotify version [1-%d, Enter=%d, q=cancel]: ", count, defaultValue))
		if strings.EqualFold(answer, "q") {
			return -1
		}
		if answer == "" {
			return recommended
		}
		index, err := strconv.Atoi(answer)
		if err == nil && index >= 1 && index <= count {
			return index - 1
		}
		fmt.Printf("Enter a number from 1 to %d, or q to cancel.\n", count)
	}
}

func (i *installer) runInstall() error {
	spotifyDir := defaultSpotifyDir()
	if spotifyDir == "" {
		return errors.New("unable to determine Spotify directory")
	}
	if i.release == nil {
		return errors.New("load the BlockTheSpot release first")
	}
	compatibility := i.release.Compatibility
	i.setStatus("Downloading BlockTheSpot")
	i.logf("Using BlockTheSpot release %s for Spotify %s.", i.release.Tag, compatibility.Version)
	files, err := i.release.downloadFiles(downloadBytes)
	if err != nil {
		return err
	}
	if err := validatePatchDLLs(files); err != nil {
		return err
	}
	if compatibility.Exact {
		settings, message, err := prepareSettings(spotifyDir, files["settings.example.ini"], i.options.ResetSettings)
		if err != nil {
			return err
		}
		files["settings.ini"] = settings
		i.logf("%s", message)
	}
	i.setProgress(15)

	spotifyExe := filepath.Join(spotifyDir, "Spotify.exe")
	selected := i.options.SpotifyVersion
	detected := ""
	if fileExists(spotifyExe) {
		detected, err = getSpotifyVersion(spotifyExe)
		if err != nil {
			i.logf("Unable to read installed Spotify version: %v", err)
		}
		i.logf("Installed Spotify version: %s.", detected)
	}
	needsInstall := i.options.UpdateSpotify || !compatibility.supports(detected) ||
		(selected.BaseVersion != "" && baseSpotifyVersion(detected) != selected.BaseVersion)
	if needsInstall && (selected.URL == "" || !compatibility.supports(selected.BaseVersion)) {
		return fmt.Errorf("Spotify %s is required by this patch release; the selected download is not compatible", compatibility.Version)
	}

	i.setStatus("Stopping Spotify")
	stopSpotifyProcesses()
	storeInstalled, err := isSpotifyStoreInstalled()
	if err != nil {
		i.logf("Warning: could not check Microsoft Store Spotify: %v", err)
	} else if storeInstalled {
		if err := uninstallSpotifyStore(); err != nil {
			return fmt.Errorf("failed to uninstall the approved Microsoft Store edition: %w", err)
		}
	}
	if needsInstall {
		i.setStatus("Installing Spotify")
		i.setProgress(20)
		if err := withPreservedSettings(spotifyDir, func() error { return i.installSpotify(spotifyExe, selected) }); err != nil {
			return err
		}
	} else {
		i.logf("Spotify update not required.")
		i.setProgress(45)
	}

	detected, err = getSpotifyVersion(spotifyExe)
	if err != nil {
		return fmt.Errorf("verify installed Spotify: %w", err)
	}
	if !compatibility.supports(detected) {
		return fmt.Errorf("installed Spotify %s does not match patch requirement %s; no patch files were installed", detected, compatibility.Version)
	}
	if err := requireWindowsX64(spotifyExe); err != nil {
		return err
	}

	i.setStatus("Applying BlockTheSpot files")
	originalPath, err := matchingOriginalDLL(spotifyDir)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(originalPath)
	if err != nil {
		return err
	}
	files["chrome_elf_required.dll"] = original
	if err := installPatchFiles(spotifyDir, files); err != nil {
		return err
	}
	i.logf("Installed patch files from %s; preferences remain in settings.ini.", i.release.Tag)
	i.setProgress(98)
	if i.options.LaunchSpotifyOnDone {
		i.setStatus("Launching Spotify")
		if err := startSpotify(spotifyExe, spotifyDir); err != nil {
			return fmt.Errorf("failed to launch Spotify: %w", err)
		}
		time.Sleep(2 * time.Second)
		running, err := processRunning("Spotify.exe")
		if err == nil && !running {
			return errors.New("Spotify did not stay running; check blockthespot.log and blockthespot-status.txt")
		}
	}
	i.setStatus("Completed")
	i.setProgress(100)
	return nil
}

func (i *installer) runUninstall() error {
	spotifyDir := defaultSpotifyDir()
	if spotifyDir == "" {
		return errors.New("unable to determine Spotify directory")
	}
	spotifyExe := filepath.Join(spotifyDir, "Spotify.exe")
	i.setStatus("Stopping Spotify")
	stopSpotifyProcesses()
	i.setStatus("Restoring Spotify")
	originalPath, err := matchingOriginalDLL(spotifyDir)
	if err != nil {
		return fmt.Errorf("cannot restore Spotify: %w", err)
	}
	if err := restoreSpotifyFiles(spotifyDir, originalPath); err != nil {
		return err
	}
	i.logf("Restored Spotify's matching original chrome_elf.dll; settings and logs were preserved.")
	if i.options.LaunchSpotifyOnDone && fileExists(spotifyExe) {
		i.setStatus("Launching Spotify")
		if err := startSpotify(spotifyExe, spotifyDir); err != nil {
			return fmt.Errorf("failed to launch Spotify: %w", err)
		}
	}
	i.setStatus("Completed")
	return nil
}

func (i *installer) installSpotify(spotifyExe string, selectedVersion spotifyInstallChoice) error {
	tempDir, err := os.MkdirTemp("", "blockthespot-spotify-setup-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	setupPath := filepath.Join(tempDir, "SpotifyFullSetupX64.exe")
	if selectedVersion.URL == "" {
		return errors.New("no compatible Spotify download selected")
	}
	i.logf("Downloading Spotify installer for %s.", selectedVersion.FullVersion)
	if err := downloadFileWithProgress(selectedVersion.URL, setupPath, selectedVersion.Size, i.logf); err != nil {
		return fmt.Errorf("failed to download Spotify installer: %w", err)
	}
	i.setProgress(35)
	i.logf("Running Spotify installer.")
	if isRunningAsAdmin() {
		if err := runInstallerViaScheduledTask(setupPath); err != nil {
			i.logf("Scheduled-task launch failed (%v); falling back to direct launch.", err)
			if err := launchDetached(setupPath); err != nil {
				return fmt.Errorf("failed to launch Spotify installer: %w", err)
			}
		}
	} else if err := launchDetached(setupPath); err != nil {
		return fmt.Errorf("failed to launch Spotify installer: %w", err)
	}
	i.logf("Waiting for Spotify setup to finish.")
	if err := waitForSpotifyInstall(spotifyExe, selectedVersion.BaseVersion, 6*time.Minute); err != nil {
		stopSpotifyProcesses()
		return err
	}
	stopSpotifyProcesses()
	i.setProgress(45)
	return nil
}

func startSpotify(exePath, workingDir string) error {
	cmd := exec.Command(exePath)
	cmd.Dir = workingDir
	return cmd.Start()
}

func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func launchDetached(filePath string) error {
	cmd := exec.Command(filePath)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func fetchSpotifyInstallChoices() (*patchRelease, []spotifyInstallChoice, int, error) {
	release, err := fetchPatchRelease(patchLatestReleaseAPI, downloadBytes)
	if err != nil {
		return nil, nil, -1, err
	}
	body, err := downloadBytes(spotifyVersionsURL)
	if err != nil {
		return release, nil, -1, fmt.Errorf("failed to download Spotify versions list: %w", err)
	}
	choices, selected, err := parseSpotifyInstallChoices(release.Compatibility, body)
	return release, choices, selected, err
}

func downloadFileWithProgress(address, targetPath string, expectedSize int64, logf func(format string, args ...any)) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	native := installerDownloadMethod{name: "Go HTTP", run: func(url, destination string, log downloadLogger) error {
		return downloadInstallerHTTP(client, url, destination, log)
	}}
	methods := []installerDownloadMethod{native}
	if curlPath, err := exec.LookPath("curl.exe"); err == nil {
		command := func(ctx context.Context, name string, args ...string) *exec.Cmd {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			return cmd
		}
		methods = []installerDownloadMethod{{name: "curl", run: func(url, destination string, log downloadLogger) error {
			return downloadInstallerCurl(command, curlPath, url, destination, log)
		}}, native}
	}
	return downloadSpotifyInstaller(address, targetPath, expectedSize, methods, logf, time.Sleep)
}

func stopSpotifyProcesses() {
	for _, name := range []string{"Spotify.exe", "SpotifyWebHelper.exe", "SpotifyFullSetup.exe", "SpotifyFullSetupX64.exe"} {
		_ = hiddenCommand("taskkill", "/IM", name, "/F").Run()
	}
}

func isSpotifyStoreInstalled() (bool, error) {
	script := "$pkg = Get-AppxPackage -Name SpotifyAB.SpotifyMusic -ErrorAction SilentlyContinue; if ($pkg) { '1' } else { '0' }"
	out, err := hiddenCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("PowerShell failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)) == "1", nil
}

func uninstallSpotifyStore() error {
	script := "$pkg = Get-AppxPackage -Name SpotifyAB.SpotifyMusic -ErrorAction SilentlyContinue; if ($pkg) { $pkg | Remove-AppxPackage -ErrorAction Stop }"
	out, err := hiddenCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("PowerShell failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isRunningAsAdmin() bool {
	script := "([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)"
	out, err := hiddenCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	return err == nil && strings.EqualFold(strings.TrimSpace(string(out)), "True")
}

func runInstallerViaScheduledTask(setupPath string) error {
	taskName := fmt.Sprintf("Spotify install %d", time.Now().UnixNano())
	escapedTaskName := strings.ReplaceAll(taskName, "'", "''")
	escapedSetupPath := strings.ReplaceAll(setupPath, "'", "''")
	script := fmt.Sprintf("$apppath='powershell.exe'; $taskname='%s'; $action=New-ScheduledTaskAction -Execute $apppath -Argument \"-NoLogo -NoProfile -Command & '%s'\"; $trigger=New-ScheduledTaskTrigger -Once -At (Get-Date); $settings=New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -WakeToRun; Register-ScheduledTask -Action $action -Trigger $trigger -TaskName $taskname -Settings $settings -Force | Out-Null; Start-ScheduledTask -TaskName $taskname; Start-Sleep -Seconds 2; Unregister-ScheduledTask -TaskName $taskname -Confirm:$false", escapedTaskName, escapedSetupPath)
	out, err := hiddenCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("PowerShell failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func getSpotifyVersion(spotifyExe string) (string, error) {
	escaped := strings.ReplaceAll(spotifyExe, "'", "''")
	script := fmt.Sprintf("$ErrorActionPreference='Stop'; $vi=(Get-Item -LiteralPath '%s').VersionInfo; '{0}.{1}.{2}.{3}' -f $vi.FileMajorPart,$vi.FileMinorPart,$vi.FileBuildPart,$vi.FilePrivatePart", escaped)
	out, err := hiddenCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("PowerShell failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	version := normalizeVersionString(strings.TrimSpace(string(out)))
	if version == "" {
		return "", errors.New("empty Spotify version")
	}
	return version, nil
}

func waitForSpotifyInstall(path, version string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if running, err := processRunning("Spotify.exe"); err == nil && running {
			installed, err := getSpotifyVersion(path)
			if err == nil && baseSpotifyVersion(installed) == version {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("Spotify setup did not start version %s before the timeout; finish setup and retry", version)
}

func processRunning(name string) (bool, error) {
	out, err := hiddenCommand("tasklist", "/FI", "IMAGENAME eq "+name, "/FO", "CSV", "/NH").CombinedOutput()
	if err != nil {
		return false, err
	}
	line := strings.TrimSpace(string(out))
	if line == "" || strings.Contains(line, "No tasks are running") {
		return false, nil
	}
	return strings.Contains(strings.ToLower(line), strings.ToLower(name)), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func defaultSpotifyDir() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "Spotify")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "AppData", "Roaming", "Spotify")
}
