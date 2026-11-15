# Changelog

All notable changes to this project are recorded here, in [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. Versions follow [SemVer 2.0.0](https://semver.org/spec/v2.0.0.html); AGENTS.md Rule 8 sets when each part changes.

## [0.1.0] — 2026-11-15

### Added

- **Docs:** README instructions for downloading the release binaries, verifying them, and running them on macOS (quarantine removal), Linux and Windows (SmartScreen, `Path`).
- **CLI:** `persadomapicli` binary built on cobra, with a `version` command that prints the version, the commit and the UTC build time.
- **Build:** root `Makefile` driving build, test, coverage, vet, format, lint and govulncheck. `make build` produces a static `CGO_ENABLED=0` binary at `src/bin/persadomapicli`.

### DevOps / CI

- **CI:** GitHub Actions workflow `ci` runs `make ci` on pushes and pull requests to `develop` and `main`, with the Go version read from `src/go.mod`.
- **Releases:** GitHub Actions workflow `release` publishes release `vX.Y.Z` on every merge into `main`, with binaries for Windows x86-64 and ARM64, Linux x86-64 and macOS Apple silicon, plus `SHA256SUMS`. `make dist` builds the same binaries locally.
- **Hooks:** `make hooks` installs a pre-commit hook that runs `make vuln` and a pre-push hook that runs `make ci`.
