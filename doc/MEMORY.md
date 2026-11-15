# Project memory

Must-Follow Rules (MFR) learned from bugs, and a session log. Keep this file under 400 lines; prune the oldest session entries first.

## Must-Follow Rules

None yet. Each MFR states the rule, the bug that prompted it and the date.

## Session log

### 2026-11-15

- Scaffolded the Go 1.27.4 module in `src/` with cobra v1.10.2 and a `version` command, at version 0.1.0.
- Added the root Makefile, the git hooks in `dev/githooks/`, the GitHub Actions `ci` workflow, AGENTS.md, CHANGELOG.md and `doc/architecture.md`.
- Renamed the binary to `persadomapicli`. Added `make dist` and the `release` workflow: every push to `main` publishes release `vX.Y.Z` with Windows amd64/arm64, Linux amd64 and macOS arm64 binaries.
- Added `install.sh` (0.1.1). Tests against a local HTTP server found three defects, now fixed: macOS 27 ships a BSD `sha256sum` without `--status`; `/dev/tty` can exist but fail to open when there is no controlling terminal; an EXIT trap that names a function-local variable fails under `set -u` after the function returns.
- `mmdc` 12.2.0 on the maintainer's Mac could not find `chrome-headless-shell`; passing `-p` with a puppeteer config whose `executablePath` points at the installed Google Chrome works (Rule 7).
