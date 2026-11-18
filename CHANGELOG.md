# Changelog

All notable changes to this project are recorded here, in [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. Versions follow [SemVer 2.0.0](https://semver.org/spec/v2.0.0.html); AGENTS.md Rule 8 sets when each part changes.

## [0.1.5] — 2026-11-18

### Added

- **release:** each release also publishes `persadomapicli-linux-arm64`, for 64-bit ARM Linux such as AWS Graviton. `install.sh` installs it on machines where `uname -m` prints `aarch64` or `arm64`.

## [0.1.4] — 2026-11-18

### Changed

- **docs:** the open work list is one file, `REMAINING_WORK_2026-11-18.md`, holding only open items; `REMAINING_WORK_2026-11-15.md` is removed.

## [0.1.3] — 2026-11-18

### Added

- **docs:** `USER_MANUAL.md` walks through every command with screenshots of real runs against the staging API; the README links to it.
- **score:** `score` scores 1 to 4 variants for predicted performance, brand fit and compliance risk (`POST /v1/score`). It waits for the job by default, or prints the job id with `--async`.
- **optimize:** `optimize` generates scored, ranked alternatives for one variant (`POST /v1/optimize`), with `--context`, `--always-use`, `--char-limit`, `--brand-file`, `--style-set`, `--exclude`, `--brand-char-limit`, `--assistant-id` and `--content-seeding`.
- **job:** `job JOB_ID` reads a score or optimize job and waits for it to finish. `--no-wait` prints it as it is, `--timeout` bounds the wait (default 15 minutes). A failed job is printed and the command exits 1.
- **performance, uplift:** `performance` predicts the performance of 1 to 10 variants. `uplift` projects the optimization potential and uplift of one variant, with `--category`.
- **catalogue, server-version:** `catalogue` lists channels, display channels, industries and limits, with `--section` to print one. `server-version` prints the API server's version, commit, build time, spec date and environment.
- **asset:** `asset create` (alias `upload`) uploads one or more files, `asset list` lists assets with `--limit`, `--cursor` and `--all`, `asset get` shows an asset's metadata and a one-hour download link, and `asset download` saves the file with `--out` and `--force`.
- **Requests:** every request command takes its input from flags, from `--file` (JSON, `-` for stdin) or both, and validates it locally before any network call.
- **Editor:** `-e/--edit` opens a built-in full-screen editor on a commented JSON template filled with the flags given. `Ctrl+S` validates and runs, `Esc` cancels, `Ctrl+R` resets, and the editor stays open until the request is valid. It needs an interactive terminal.
- **Authentication:** the API key comes from `--api-key`, `--api-key-file` or `PERSADO_API_KEY`, and the region from `--base-url` or `PERSADO_API_URL` (default `https://api.persado.com/api`).
- **Output:** `-o json` prints the server's JSON unchanged. `--lang` sets the language of localized text.
- **API client:** a request that gets `429 rate_limited` or `503` is resent up to 3 times, honoring `Retry-After` for at most 30 seconds per wait. API errors show the status, code, invalid fields and request id.
- **Docs:** README usage section, request-flow, editor and error-model sections in `doc/architecture.md`, and two new diagrams.

## [0.1.2] — 2026-11-15

### Added

- **Update check:** every run checks GitHub for a newer release in the background. If one exists, the tool prints the upgrade one-liner for the current OS on stderr before it exits, waiting at most 2 seconds for the check.
- **API banner:** commands that call the Martech API print `Persado Martech API <version>-<commit>` on stderr before they run.
- **Install:** `install.ps1` installs or upgrades the latest release on Windows x86-64 and ARM64 with `irm https://raw.githubusercontent.com/persado/martechapi-cli/main/install.ps1 | iex`, without administrator rights.

### Changed

- **version:** the output now starts with `Persado Martech API`, for example `Persado Martech API 0.1.2 (commit fed3a10, built 2026-11-15T09:53:19Z)`.

## [0.1.1] — 2026-11-15

### Added

- **Install:** `install.sh` installs the latest release on macOS (Apple silicon) and Linux (x86-64) with one `curl | bash` command. It verifies the SHA-256 checksum, clears the macOS quarantine attribute, and installs to `/usr/local/bin` after asking for confirmation when sudo is needed, or to `~/.local/bin` without sudo. The README opens with it as the Quickstart.

## [0.1.0] — 2026-11-15

### Added

- **Docs:** README instructions for downloading the release binaries, verifying them, and running them on macOS (quarantine removal), Linux and Windows (SmartScreen, `Path`).
- **CLI:** `persadomapicli` binary built on cobra, with a `version` command that prints the version, the commit and the UTC build time.
- **Build:** root `Makefile` driving build, test, coverage, vet, format, lint and govulncheck. `make build` produces a static `CGO_ENABLED=0` binary at `src/bin/persadomapicli`.

### DevOps / CI

- **CI:** GitHub Actions workflow `ci` runs `make ci` on pushes and pull requests to `develop` and `main`, with the Go version read from `src/go.mod`.
- **Releases:** GitHub Actions workflow `release` publishes release `vX.Y.Z` on every merge into `main`, with binaries for Windows x86-64 and ARM64, Linux x86-64 and macOS Apple silicon, plus `SHA256SUMS`. `make dist` builds the same binaries locally.
- **Hooks:** `make hooks` installs a pre-commit hook that runs `make vuln` and a pre-push hook that runs `make ci`.
