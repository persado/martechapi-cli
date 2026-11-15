# martechapi-cli

`persadomapicli` is the Persado Martech API command-line tool, free to use. It uploads assets in batch for processing by the Martech API, for APIs that take multiple assets, such as Market Scan.

This release contains the project scaffold and a `version` command; the upload commands are still to be added. The Martech API Terms of Service govern how uploaded assets are processed, stored and retained.

## Quickstart

On macOS (Apple silicon) or Linux (x86-64), run:

```sh
curl -fsSL https://raw.githubusercontent.com/persado/martechapi-cli/main/install.sh | bash
```

The [install script](install.sh) does the following:

1. Picks the release asset for your OS and processor, including on a Rosetta shell on Apple silicon.
2. Downloads it and `SHA256SUMS` from the latest release, and stops if the checksum does not match.
3. On macOS, removes the quarantine attribute so that Gatekeeper does not block the binary.
4. Installs `persadomapicli` to `/usr/local/bin`. If that directory is not writable by your user, the script says that sudo (administrator) access is required and asks you to confirm before sudo prompts for your password. If you decline, or no terminal is available, it installs to `~/.local/bin` without sudo.
5. Runs `persadomapicli version`, and prints the line to add to your shell profile if the install directory is not on your `PATH`.

On Windows (x86-64 or ARM64), run in PowerShell:

```powershell
irm https://raw.githubusercontent.com/persado/martechapi-cli/main/install.ps1 | iex
```

The [Windows install script](install.ps1) needs no administrator rights. It picks the asset for your processor, verifies its checksum, installs `persadomapicli.exe` to `%LOCALAPPDATA%\Programs\persadomapicli`, removes the "downloaded from the internet" mark that triggers SmartScreen, and adds that folder to your user `Path`.

Running either command again upgrades an existing install to the latest release. To inspect the script before running it, download it first: `curl -fsSLO https://raw.githubusercontent.com/persado/martechapi-cli/main/install.sh`, then `bash install.sh`.

## Install from a release

The steps below install a release by hand.


Download the binary for your platform from the [latest release](https://github.com/persado/martechapi-cli/releases/latest). Each binary is a single executable with no installer and no dependencies.

| Platform | Asset |
|---|---|
| macOS, Apple silicon (M1 and later) | `persadomapicli-macos-arm64` |
| Linux, x86-64 | `persadomapicli-linux-amd64` |
| Windows, x86-64 | `persadomapicli-windows-amd64.exe` |
| Windows, ARM64 | `persadomapicli-windows-arm64.exe` |

Asset names carry no version number, so this link always serves the newest release:

```text
https://github.com/persado/martechapi-cli/releases/latest/download/<asset>
```

Each release also contains `SHA256SUMS`, the SHA-256 checksum of every asset (see [Verify a download](#verify-a-download)). No build is published for Intel Macs or ARM Linux; on those platforms, [build from source](#build-from-source).

### macOS

```sh
curl -fLO https://github.com/persado/martechapi-cli/releases/latest/download/persadomapicli-macos-arm64
chmod +x persadomapicli-macos-arm64
sudo mv persadomapicli-macos-arm64 /usr/local/bin/persadomapicli
persadomapicli version
```

The binary is not notarized by Apple. When it is downloaded with a web browser, macOS sets the `com.apple.quarantine` attribute on the file, and Gatekeeper refuses to run it. Remove the attribute once, after the download:

```sh
xattr -d com.apple.quarantine /usr/local/bin/persadomapicli
```

`xattr -l /usr/local/bin/persadomapicli` lists the attributes still set. A file downloaded with `curl`, as above, does not get the attribute, and `xattr -d` then reports `No such xattr`, which is harmless. Alternatively, after macOS has blocked the first run, open **System Settings > Privacy & Security** and click **Open Anyway** next to the message about `persadomapicli`.

### Linux

```sh
curl -fLO https://github.com/persado/martechapi-cli/releases/latest/download/persadomapicli-linux-amd64
sudo install -m 0755 persadomapicli-linux-amd64 /usr/local/bin/persadomapicli
persadomapicli version
```

The binary is statically linked and runs on any x86-64 distribution.

### Windows

Check the processor architecture in PowerShell; it prints `AMD64` or `ARM64`:

```powershell
$env:PROCESSOR_ARCHITECTURE
```

Download the matching asset (replace `amd64` with `arm64` on an ARM64 machine) and run it:

```powershell
Invoke-WebRequest -Uri https://github.com/persado/martechapi-cli/releases/latest/download/persadomapicli-windows-amd64.exe -OutFile persadomapicli.exe
Unblock-File .\persadomapicli.exe
.\persadomapicli.exe version
```

The binary is not code-signed. A file downloaded with a web browser carries the "downloaded from the internet" mark, and SmartScreen may show **Windows protected your PC** on first run. `Unblock-File` removes the mark; in the SmartScreen dialog, **More info > Run anyway** has the same effect.

To run `persadomapicli` from any directory, move it to a folder of its own and add that folder to the user `Path`, then open a new terminal:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\persadomapicli"
New-Item -ItemType Directory -Force $dir | Out-Null
Move-Item .\persadomapicli.exe $dir -Force
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

### Verify a download

Download `SHA256SUMS` into the same directory as the asset, and check the asset before renaming it.

macOS and Linux:

```sh
curl -fLO https://github.com/persado/martechapi-cli/releases/latest/download/SHA256SUMS
shasum -a 256 -c --ignore-missing SHA256SUMS     # Linux: sha256sum -c --ignore-missing SHA256SUMS
```

Windows (PowerShell), for the asset as downloaded under its original name; the last line prints `True` when the checksum matches:

```powershell
Invoke-WebRequest -Uri https://github.com/persado/martechapi-cli/releases/latest/download/SHA256SUMS -OutFile SHA256SUMS
$asset = "persadomapicli-windows-amd64.exe"
$expected = (Select-String -Path SHA256SUMS -Pattern $asset -SimpleMatch).Line.Split(" ")[0]
(Get-FileHash .\$asset -Algorithm SHA256).Hash -eq $expected
```

## Usage

```sh
persadomapicli --help      # list commands
persadomapicli version     # print "Persado Martech API", the version, commit and build time
```

Commands that call the Martech API first print `Persado Martech API <version>-<commit>` (for example `Persado Martech API 0.1.2-fed3a10`) on stderr.

Each run checks for a newer release in the background with one HTTPS `HEAD` request to `https://github.com/persado/martechapi-cli/releases/latest`, which sends your IP address and the user agent `persadomapicli/<version>` to GitHub. If a newer release exists, the tool prints the upgrade command on stderr just before it exits. When the command finishes before the check, the tool waits at most 2 seconds for it; if GitHub is unreachable, it prints nothing.

## Build from source

Prerequisites:

- Go 1.27.4 (the `go` line of `src/go.mod`)
- GNU Make and git
- For development only: network access on the first `make lint` and `make vuln`, which build golangci-lint v2.15.0 and govulncheck v1.9.0 with `go run`

```sh
git clone https://github.com/persado/martechapi-cli.git
cd martechapi-cli
make build
./src/bin/persadomapicli version
```

`make` with no target lists every target. `make dist` cross-compiles the four release binaries into `src/dist/`.

## Development

```sh
make hooks   # once per clone: pre-commit runs make vuln, pre-push runs make ci
make all     # fmt-check + vet + test + build
make ci      # the gates CI runs
```

A release is published when a pull request from `develop` is merged into `main`: the `release` workflow runs `make ci`, builds the four binaries, and publishes them as release `vX.Y.Z`, where `X.Y.Z` is `const Version` in `src/pkg/version/version.go`. The release notes are that version's section of the CHANGELOG.

Contributor and agent rules are in [AGENTS.md](AGENTS.md); the change history is in [CHANGELOG.md](CHANGELOG.md).

## Help

Open an issue at <https://github.com/persado/martechapi-cli/issues>.

## License

BSD 3-Clause; see [LICENSE](LICENSE).
