# martechapi-cli

`persadomapicli` is the Persado Martech API command-line tool, free to use. It scores and optimizes marketing copy, predicts performance and uplift, and uploads, lists and downloads assets, all through the Martech API.

The Martech API Terms of Service govern how uploaded assets are processed, stored and retained.

## Quickstart

On macOS (Apple silicon) or Linux (x86-64 or ARM64), run:

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
| Linux, ARM64 (AWS Graviton, Ampere, Raspberry Pi 4 and later with a 64-bit OS) | `persadomapicli-linux-arm64` |
| Windows, x86-64 | `persadomapicli-windows-amd64.exe` |
| Windows, ARM64 | `persadomapicli-windows-arm64.exe` |

Asset names carry no version number, so this link always serves the newest release:

```text
https://github.com/persado/martechapi-cli/releases/latest/download/<asset>
```

Each release also contains `SHA256SUMS`, the SHA-256 checksum of every asset (see [Verify a download](#verify-a-download)). No build is published for Intel Macs or 32-bit ARM Linux; on those platforms, [build from source](#build-from-source).

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

On an ARM64 machine (`uname -m` prints `aarch64`), replace `amd64` with `arm64`. The binary is statically linked and runs on any x86-64 or ARM64 distribution.

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

The [user manual](USER_MANUAL.md) covers every command step by step, with screenshots of real runs. This section is a summary.

```sh
persadomapicli --help               # list commands
persadomapicli COMMAND --help       # description, limits and examples of one command
persadomapicli version              # print "Persado Martech API", the version, commit and build time
```

### Authentication and region

Every command that calls the API needs an API key (`pmk_...`). The tool reads it from the first of these that is set:

1. `--api-key KEY`
2. `--api-key-file FILE`, a file that holds the key (surrounding whitespace is ignored)
3. the environment variable `PERSADO_API_KEY`

```sh
export PERSADO_API_KEY=pmk_...
persadomapicli server-version
```

A key works only in the region that issued it. Choose the region with `--base-url URL` or the environment variable `PERSADO_API_URL`; the default is `https://api.persado.com/api`.

| Region | Base URL |
|---|---|
| `api` (default) | `https://api.persado.com/api` |
| `eapi` | `https://eapi.persado.com/api` |
| `gapi` | `https://gapi.persado.com/api` |

Flags available on every command:

| Flag | Meaning |
|---|---|
| `--api-key`, `--api-key-file`, `--base-url` | Key and region, as above. |
| `--lang CODE` | Language of localized text in the response (`Accept-Language`), such as `en` or `fr`. |
| `-o, --output text\|json` | `text` (default) prints tables. `json` prints the server's JSON response unchanged. |

The tool resends a request up to 3 times when the API answers `429 rate_limited` or `503`, waiting for the server's `Retry-After` (at most 30 seconds per wait).

### Commands

| Command | What it does | Example |
|---|---|---|
| `server-version` | Prints the API server's version, commit, build time, spec date and environment. | `persadomapicli server-version` |
| `catalogue` | Lists channels, display channels, industries and limits. `--section` prints one section. | `persadomapicli catalogue --section channels` |
| `score` | Scores 1 to 4 variants for predicted performance, brand fit and compliance risk. | `persadomapicli score -a "Acme Bank" -v "Your card is waiting" -v "Apply in minutes"` |
| `optimize` | Generates scored, ranked alternatives for one variant. | `persadomapicli optimize -a "Acme Bank" -v "Your card is waiting" --context "0% intro APR for 15 months"` |
| `job JOB_ID` | Reads a score or optimize job and waits for it to finish. Jobs are kept for 24 hours. | `persadomapicli job job_0123456789abcdef0123456789abcdef` |
| `performance` | Predicts the performance of 1 to 10 variants. | `persadomapicli performance -a "Acme Bank" -v "Your card is waiting"` |
| `uplift` | Projects the optimization potential and uplift of one variant. | `persadomapicli uplift -a "Acme Bank" -v "Apply today" --category click` |
| `asset create FILE...` | Uploads files as assets (alias `asset upload`). Expiry 1 to 90 days, default 30. | `persadomapicli asset create brief.pdf --expires-in-days 7 --attr campaign=spring` |
| `asset list` | Lists your assets, newest first. `--limit`, `--cursor`, `--all`. | `persadomapicli asset list --all` |
| `asset get ASSET_ID` | Shows an asset's metadata and a one-hour download link. | `persadomapicli asset get 0b9a3a8e-5d1f-4c53-9a77-3f2d6c1e8b40` |
| `asset download ASSET_ID` | Saves the asset's file. `--out PATH` or `--out -` for stdout, `--force` to overwrite. | `persadomapicli asset download 0b9a3a8e-5d1f-4c53-9a77-3f2d6c1e8b40 --out brief.pdf` |
| `version` | Prints the tool's own version. | `persadomapicli version` |

`score`, `optimize`, `performance` and `uplift` share these flags: `-a/--account` (the brand, required), `-c/--channel`, `--display-channel`, `--industry`, `--country`, `--region`, `--language`, `--parent-id`, `-v/--variant` (plain text, or a JSON object of touchpoints such as `'{"headline":"Hi","body":"Apply today"}'`), `-f/--file` and `-e/--edit`. `optimize` adds `--context`, `--always-use`, `--char-limit`, `--brand-file`, `--style-set`, `--exclude`, `--brand-char-limit`, `--assistant-id` and `--content-seeding`. `uplift` adds `--category`. Run `COMMAND --help` for the allowed values and limits.

### Jobs

`score` and `optimize` run as jobs. By default the request asks the API to hold the connection for up to 50 seconds. If the job is still running after that, the command polls it every 2 seconds, and prints the result when it completes. `--async` prints the job id as soon as the API accepts the request, and `persadomapicli job JOB_ID` reads it later. `--timeout` (default 15 minutes) bounds the wait; when it passes, the command exits with an error that names the job so that you can read it later. `job --no-wait` prints the job as it is now.

### Requests from a file, from stdin or from the editor

A request comes from flags alone, from a JSON file (`--file request.json`, or `--file -` for stdin), or from both: flags you pass override the file. The JSON is strict, so an unknown field is an error. Every request is validated locally before any network call, and the error names the field and the fix.

```sh
persadomapicli score --file request.json -o json
cat request.json | persadomapicli score --file - 
```

### Editing a request with `--edit`

`-e/--edit` opens a full-screen editor in the terminal, with no external program, so it behaves the same on macOS, Linux and Windows. It needs an interactive terminal. The editor starts from a JSON template filled with the flags you passed, and lists the allowed values in lines that start with `//`; the tool ignores those lines.

| Key | Action |
|---|---|
| `Ctrl+S` | Validate and run. If the request is invalid, the editor stays open and shows the error on the status line. |
| `Esc` | Cancel. If you changed the text, the editor asks you to confirm with `Y`. |
| `Ctrl+R` | Reset the text to the template. |
| `Ctrl+K` | Delete to the end of the line. |

```sh
persadomapicli score -a "Acme Bank" --edit
```

### Output and exit codes

Command output goes to stdout. The banner, job progress, update notices and errors go to stderr, so `persadomapicli ... -o json > out.json` captures clean JSON. Errors print as `error: <message>`; an API error also shows the HTTP status, the error code, the invalid fields and the request id.

| Exit code | Meaning |
|---|---|
| 0 | Success. |
| 1 | Any error: invalid input, API error, `--edit` canceled, timeout, or a job that finished as failed (the job is printed first). |

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

A release is published when a pull request from `develop` is merged into `main`: the `release` workflow runs `make ci`, builds the release binaries, and publishes them as release `vX.Y.Z`, where `X.Y.Z` is `const Version` in `src/pkg/version/version.go`. The release notes are that version's section of the CHANGELOG.

Contributor and agent rules are in [AGENTS.md](AGENTS.md); the change history is in [CHANGELOG.md](CHANGELOG.md).

## Help

Open an issue at <https://github.com/persado/martechapi-cli/issues>.

## License

BSD 3-Clause; see [LICENSE](LICENSE).
