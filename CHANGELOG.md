# Changelog

All notable changes to this project are recorded here, in [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. Versions follow [SemVer 2.0.0](https://semver.org/spec/v2.0.0.html); AGENTS.md Rule 8 sets when each part changes.

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
