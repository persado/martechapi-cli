# Project memory

Must-Follow Rules (MFR) learned from bugs, and a session log. Keep this file under 400 lines; prune the oldest session entries first.

## Must-Follow Rules

Each MFR states the rule, the bug that prompted it and the date.

- **MFR-1 (2026-11-18):** a field the spec makes required whenever its parent is present is a pointer. Bug: `generation.contentSeedingEnabled` as a plain `bool` sent `false` when the user gave no value. AGENTS.md Rule 13.
- **MFR-2 (2026-11-18):** a wait that hits `--timeout` during an in-flight request returns the "still running" message, not `context deadline exceeded`. AGENTS.md Rule 14.
- **MFR-3 (2026-11-18):** an upload never sends an absolute local path as the asset filename. Bug: `asset create /Users/<name>/x.pdf` stored the full path. AGENTS.md Rule 14.

## Session log

### 2026-11-18

- 0.1.3: added the API commands (`score`, `optimize`, `job`, `performance`, `uplift`, `catalogue`, `server-version`, `asset create|list|get|download`), the `pkg/api` client with retries, and the `pkg/editor` Bubble Tea editor behind `--edit`. Developer-key endpoints (`/v1/devkeys/*`) are out of scope.
- Built by a workflow: 9 implementers, 4 review agents (spec, UX, error handling, quality), per-file fixers and a CI gate; coverage 96.7 %. Lessons became AGENTS.md Rules 11 to 16.
- Documented in the README, `doc/architecture.md` and the new diagrams `command-flow` and `edit-flow`, rendered with `mmdc` and a puppeteer config (Rule 7).

### 2026-11-15

- Scaffolded the Go 1.27.4 module in `src/` with cobra v1.10.2 and a `version` command, at version 0.1.0.
- Added the root Makefile, the git hooks in `dev/githooks/`, the GitHub Actions `ci` workflow, AGENTS.md, CHANGELOG.md and `doc/architecture.md`.
- Renamed the binary to `persadomapicli`. Added `make dist` and the `release` workflow: every push to `main` publishes release `vX.Y.Z` with Windows amd64/arm64, Linux amd64 and macOS arm64 binaries.
- Added `install.sh` (0.1.1). Tests against a local HTTP server found three defects, now fixed: macOS 27 ships a BSD `sha256sum` without `--status`; `/dev/tty` can exist but fail to open when there is no controlling terminal; an EXIT trap that names a function-local variable fails under `set -u` after the function returns.
- 0.1.2: `version` prints the product name; API commands print a banner; background update check with a 2 s cap at exit; `install.ps1` for Windows. `install.ps1` was tested under `irm | iex` in the `mcr.microsoft.com/powershell` container (amd64 only, run with `--platform linux/amd64` on Colima, files staged under `$HOME` because Colima shares only the home directory). Windows-only behaviour (`Unblock-File`, user `Path`) was stubbed and remains untested on real Windows.
- `mmdc` 12.2.0 on the maintainer's Mac could not find `chrome-headless-shell`; passing `-p` with a puppeteer config whose `executablePath` points at the installed Google Chrome works (Rule 7).
