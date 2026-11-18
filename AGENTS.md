# AGENTS.md

Instructions for coding agents and contributors working in this repository. Sections 1 to 4 describe the project; the RULES that follow are binding on every change.

## 1. Project Overview

This is the Persado Martech API Commandline Tool. This tool is available for free, to allow users of the Martech API to quickly and efficiently upload assets for processing with the API, on a batch mode, suitable for e.g. access to the Market Scan or similar advanced APIs that require multiple assets. For details on asset manipulation, storage and retainment, check the related ToS of the Martech API.

- **What it is:** `martechapi-cli`, a Go command-line application. The binary is `persadomapicli`; the command tree uses [cobra](https://github.com/spf13/cobra) v1.10.2.
- **Toolchain:** Go 1.27.4, set by the `go` line of `src/go.mod`. CI reads the version from that file; `make go-bump V=X.Y.Z` moves it.
- **Layout:** the Go module (`github.com/persado/martechapi-cli/src`) lives in `src/`: entry point in `src/cmd/persadomapicli/`, commands in `src/pkg/cli/`, API client in `src/pkg/api/`, `--edit` editor in `src/pkg/editor/`, version in `src/pkg/version/`. Details in [doc/architecture.md](doc/architecture.md).
- **Branches:** `develop` is the default branch and takes direct commits or merged PRs. `main` is protected: changes arrive by pull request only, and force pushes and deletion are blocked.
- **CI:** GitHub Actions workflow `.github/workflows/ci.yml` runs `make ci` on pushes and pull requests to `develop` and `main`.
- **Releases:** `.github/workflows/release.yml` runs on every push to `main` (a merged `develop` → `main` PR). It runs `make ci` and `make dist`, then publishes GitHub Release `vX.Y.Z` from `const Version`, with the CHANGELOG section as notes. Assets: `persadomapicli-windows-amd64.exe`, `persadomapicli-windows-arm64.exe`, `persadomapicli-linux-amd64`, `persadomapicli-linux-arm64`, `persadomapicli-macos-arm64` and `SHA256SUMS`. The run fails if release `vX.Y.Z` already exists, so a merge into `main` needs a version not yet released (Rule 8).
- **Installers:** `install.sh` (macOS, Linux: `curl -fsSL https://raw.githubusercontent.com/persado/martechapi-cli/main/install.sh | bash`) and `install.ps1` (Windows: `irm https://raw.githubusercontent.com/persado/martechapi-cli/main/install.ps1 | iex`) at the repository root. Both are served from `main`, so a change reaches users only on the next release. They hard-code the asset names from `make dist`; renaming an asset or changing `PLATFORMS` requires the same change in both scripts, the README, and the one-liners in `src/pkg/update/update.go`. `install.ps1` reports errors with `throw`, never `exit`, because `exit` under `iex` closes the user's shell.
- **Update check:** every run sends one `HEAD` request to `https://github.com/persado/martechapi-cli/releases/latest` in the background and reads the tag from the redirect. This avoids the REST API limit of 60 unauthenticated requests per hour per IP address. `cli.Run` waits at most 2 s (`updateWait`) at exit for it, then prints the upgrade notice on stderr if a newer release exists.
- **License:** BSD 3-Clause.

## 2. Build and Test Commands

The Go module lives in `src/`; the root `Makefile` drives all operations.

```sh
make              # prints help (.DEFAULT_GOAL := help) — it does not build
make build        # static CGO_ENABLED=0 binary -> src/bin/persadomapicli (commit/build-time stamped)
make dist         # cross-compile the release binaries (PLATFORMS) -> src/dist/ + SHA256SUMS
make test         # go test ./... -count=1
make test-race    # CGO_ENABLED=1 go test -race ./... -count=1
make cover        # coverage profile (src/coverage.out) + total line
make cover-html   # HTML report -> src/coverage.html
make cover-check  # fail if TOTAL coverage (run with -race) < 70% (COVER_MIN)
make vet          # go vet ./...
make fmt          # gofmt -w .
make fmt-check    # fail if any file is not gofmt-clean
make lint         # golangci-lint at GOLANGCI_LINT_VERSION, built with the go.mod Go via go run (no .golangci.yml exists — defaults apply)
make go-bump V=X.Y.Z  # move the go line of src/go.mod to Go X.Y.Z; CI follows it
make vuln         # govulncheck ./... (pinned GOVULNCHECK_VERSION) — fails on any vulnerability reachable from our code (Rule 10)
make hooks        # git config core.hooksPath dev/githooks — installs the pre-commit govulncheck gate and the pre-push gate
make prepush      # run the pre-push gate (make ci) without pushing; git push already runs it through the hook
make tidy         # go mod tidy
make verify       # go mod verify
make download     # go mod download
make clean        # rm -rf src/bin src/dist src/coverage.out src/coverage.html
make all          # fmt-check + vet + test + build (local default)
make ci           # fmt-check + vet + lint + vuln + cover-check (with -race) + build
```

## 3. Code Style Guidelines

- **Formatter:** `gofmt` (standard Go style), enforced by `make fmt-check`. There is no custom formatter config.
- **Vet:** `go vet ./...` must pass clean (`make vet`).
- **Linter:** `golangci-lint` with **default rules** (no `.golangci.yml` is committed, so its built-in linter set applies).
- **Conventions (observed in code):**
  - Commands write output through `cmd.OutOrStdout()` / `cmd.ErrOrStderr()`, never directly to `os.Stdout`, so tests capture it.
  - Commands return errors from `RunE`; they do not print and exit. `cli.Run` prints `error: <message>` on stderr and returns exit code 1, and `main` is the only caller of `os.Exit`.
  - Errors are wrapped with `fmt.Errorf("...: %w", err)` and propagated, not panicked.
  - Each command has its own file in `src/pkg/cli/` with an unexported `new<Command>Cmd()` constructor, registered in `newRootCmd`.
  - A command that calls the Martech API sets `Annotations: map[string]string{annotationAPI: ""}`; the root's `PersistentPreRunE` then prints `version.Banner()` (`Persado Martech API 0.1.2-fed3a10`) on stderr. stdout carries only command output.

## 4. Testing Instructions

- Run `make test` while iterating and `make test-race` before committing. `make ci` is the full gate CI applies.
- Tests live next to the file they test, one to one (Rule 5).
- Test commands end to end through `cli.Run(ctx, args, &stdout, &stderr)` with `bytes.Buffer` writers, and assert on the exit code and the output. Do not call `os.Exit` or touch the real stdout in tests.
- Prefer table-driven tests with `t.Run` subtests. Tests must not reach the network: `src/pkg/cli/cli_test.go` stubs `updateCheck` for the whole package in `TestMain`, and `src/pkg/update` tests use `httptest` servers.
- A test that changes a package variable (for example `version.Commit`) restores it with `t.Cleanup`.
- `make cover-check` fails CI below 70 % total coverage; the target for changed packages is 80 % (Pre-Commit Checklist).

# RULES

## 0. Be token frugal in your responses : **Don't blabber around. Don't elaborate on solutions unless clearly asked. I understand a lot.**

## 1. Think Before Coding : **Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First **Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes **Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution **Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## 5. Per-File Test Files **Tests live next to the file they test, one-to-one.**

Every non-trivial source file under `src/` **must** have a dedicated `_test.go` companion with the same base name:

| Source file | Required test file |
|---|---|
| `src/pkg/cli/root.go` | `src/pkg/cli/root_test.go` |
| `src/pkg/cli/version.go` | `src/pkg/cli/version_test.go` |
| `src/pkg/version/version.go` | `src/pkg/version/version_test.go` |
| `src/pkg/update/update.go` | `src/pkg/update/update_test.go` |

`src/cmd/persadomapicli/main.go` is exempt: it only calls `cli.Run` and `os.Exit`.

**Rules:**
- A catch-all `*_test.go` **must not** be the only test file for a package that contains several source files. It may exist for package-level fixtures and helpers only.
- Each per-file test covers **only** the functions defined in its paired source file.
- When a new source file is created, its `_test.go` must be created in the **same commit/PR**.
- When a function is moved between files, its tests move with it.
- Agents: when asked to create or modify a command, always check whether the paired `_test.go` exists and create/update it as part of the same task. Never deliver a command change without a paired test file update.

## 6. Keep the CHANGELOG current **Every user-facing change updates `CHANGELOG.md` in the same commit/PR.**

The project maintains a root [`CHANGELOG.md`](CHANGELOG.md) in
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. It is the human-readable record
of what changed and why: treat it as part of the deliverable, not an afterthought.

**When to add an entry.** Any change that a user, operator, or integrator would notice:
new/changed/removed commands, flags, arguments, output formats or exit codes, config/env vars,
API behavior, build, CI or release changes, and bug fixes.
Purely internal refactors with no observable effect, test-only changes, and formatting do **not**
need an entry.

**How to add an entry.**
- Add it under the version section of the commit or PR that lands it: `## [X.Y.Z] — YYYY-MM-DD`,
  where `X.Y.Z` is the version that same change sets in `version.go` (Rule 8). The newest section
  goes on top.
- Use one of the standard subsections: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`,
  `Security` (plus `DevOps / CI` for pipeline/deploy work). Create the subsection under that
  version section if it does not exist yet.
- One bullet per change, present tense, written for a reader who did **not** see the diff. Lead with
  the area in **bold** where it helps (e.g. `**version:** ...`). Reference the ticket (e.g.
  `SEN-12345`) when there is one.
- Do not paste raw commit subjects; summarize the user-visible effect.

**No `[Unreleased]` section.** Every landing on `develop` is a version (Rule 8), so there is nothing
unreleased to collect. Keep sections and entries newest-first.

**Enforcement for agents:** when a task changes anything user-facing, update `CHANGELOG.md` in the
same commit as the code. A user-facing change with no CHANGELOG entry is a review blocker.

## 7. Mermaid diagrams are verified with mermaid-cli **A diagram nobody rendered is a diagram nobody checked.**

Every Mermaid diagram lives in `doc/diagrams/` as its own source, `doc/diagrams/<name>.mmd` (kebab-case), with the
rendered `<name>.png` beside it. Do not write a ```` ```mermaid ```` block into a Markdown file. Create the `.mmd`,
render it, then embed the PNG and link the source:

```markdown
![<caption>](diagrams/<name>.png)

<sub>Diagram source: [diagrams/<name>.mmd](diagrams/<name>.mmd)</sub>
```

From a file in `doc/archive/` the path is `../diagrams/`; from a root file it is `doc/diagrams/`. Render each source with
the repository's config, which stays at `doc/mermaid-config.json`:

```sh
mmdc -q -c doc/mermaid-config.json -i doc/diagrams/<name>.mmd -o doc/diagrams/<name>.png -b white -s 2
```

If `mmdc` reports that it cannot find `chrome-headless-shell`, add `-p <file>` with a puppeteer config such as
`{"executablePath": "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}`.

A non-zero exit is a parse error: fix the diagram, do not ship it. When you change an embedded diagram, edit its `.mmd`
and render it again. GitHub and GitLab render a broken block as an error box, and nothing else in the toolchain checks
Mermaid syntax.

The failure that prompted this rule: a `;` inside a sequence-diagram message ends the statement in Mermaid's grammar,
so `A->>B: batch; then per variant` fails to parse. It reads fine as prose and only a render catches it.

## 8. Semantic versioning **Every commit or PR that lands on `develop` increments the patch.**

The version is `MAJOR.MINOR.PATCH` ([SemVer 2.0.0](https://semver.org/spec/v2.0.0.html)), held in one place:
`const Version` in `src/pkg/version/version.go`. It is what `persadomapicli version` prints.

- **PATCH +1 per landing on `develop`:** `0.70.0` → `0.70.1` → `0.70.2`. A landing is one direct commit to
  `develop` or one merged PR. A PR bumps once, however many commits it has; the bump is a commit inside it.
- **MINOR or MAJOR only when the maintainer asks.** Either resets PATCH to `0` (`0.70.4` → `0.71.0`).
- **The number never goes down.** Compare each part as an integer: `0.70.0` > `0.65.5` > `0.7.0`.
- **The same change opens its CHANGELOG section**, `## [X.Y.Z] — YYYY-MM-DD`, carrying its entries (Rule 6).
- **Two PRs in flight:** whichever merges second rebases onto `develop` and takes the next free number;
  its version and CHANGELOG section move with it.

## 9. Remaining work **always saved in a markdown document.**

The remaining work ("still open" or "bugs found but not created by me" or "still pending) should never stay as the last thing in context; they should always land in `./REMAINING_WORK_<ISODate>.md` and clearly mentioned. Keeping this in a long context (and paying a price for it when resuming) is not sensible.

## 10. govulncheck gates every commit **No commit, and no merge, with a known vulnerability reachable from our code.**

`make vuln` runs `govulncheck ./...` (version pinned by `GOVULNCHECK_VERSION` in the Makefile) and fails when a vulnerability in the Go toolchain or a module is **reachable** from our code; findings in modules we require but never call are reported without failing.

- **Always run it before committing.** The `pre-commit` hook in `dev/githooks/` runs `make vuln`; install it with `make hooks` on every clone. Agents run `make vuln` themselves before every commit, hook or not, and never bypass it with `--no-verify`.
- **It gates CI.** The GitHub Actions `ci` job runs `make ci`, which includes `vuln`, so a PR with a reachable vulnerability does not go green.
- **A finding is fixed, not waived.** Upgrade the module or the Go toolchain it names: for a stdlib finding, run `make go-bump V=X.Y.Z`; CI picks up the new version from `src/go.mod`. If no fixed version exists yet, stop and ask; do not merge around it.
- New vulnerabilities are published independently of our changes, so a commit that touches nothing related can still fail the gate. That is the point: fix it in its own commit first.

## 11. API command shape **Every command that calls the Martech API is built the same way as `score.go`.**

- The request is built from `--file` (strict JSON, `-` for stdin), then only the flags the user changed override it, then `finalize` validates it or opens the editor. No request leaves the machine before the local `Validate()` passes.
- Every command that takes input accepts `-e/--edit`. Its template is commented JSON (`//` lines): it lists the allowed values, pre-fills the flag values and carries every required field, with empty values where the user gave none.
- Text output goes to stdout as tabwriter tables with title-case headers (`Filename`, not `FILENAME`) and `-` for a null value. `-o json` prints the server's JSON unchanged. Progress, the banner and errors go to stderr.
- Scores are printed at the precision the API returns: two decimals for `performance` and `uplift`, integers for job variants.
- The "Developer keys" endpoints (`/v1/devkeys/*`) are out of scope unless the maintainer asks for them.

## 12. The `--edit` editor is built in **It never starts `$EDITOR` or any other external program.**

- `src/pkg/editor` is a Bubble Tea v2 textarea. It shows its keys on screen at all times (Ctrl+S save and run, Esc cancel, Ctrl+R reset), shows a validation error without closing, and asks before discarding changes.
- It must not depend on OS support: the textarea's clipboard paste binding stays disabled (it shells out to `pbpaste`/`xclip`); terminal paste still works.
- `--edit` without an interactive terminal fails with an error. Tests replace `runEditor` and never open a terminal.

## 13. Validation mirrors the spec exactly **A field the spec makes conditionally required is a pointer.**

- When the spec says a field is required whenever its parent is present (`generation.contentSeedingEnabled`), the Go field is a pointer, so that a missing value differs from `false`. A plain `bool` silently sent `false`.
- An enum check must reject the empty string where the spec has no empty value: `responseCategories: [""]` passed locally and failed on the server.
- Check a media type with `mime.ParseMediaType`, as the server does. A test for `contains "/"` accepted `text/`.

Lessons from the 0.1.3 review, 2026-11-18.

## 14. Bounded I/O **Every network call and file write has a bound and a clean failure.**

- `main` cancels the context on SIGINT and SIGTERM (`signal.NotifyContext`), so deferred cleanup runs on Ctrl-C.
- The HTTP client has a timeout (`httpTimeout`); `http.DefaultClient` has none and hangs on a stalled server.
- A wait or poll that hits its `--timeout` returns the actionable message ("still running after X; read it later with ..."), including when the deadline expires during an in-flight request, never the raw `context deadline exceeded`.
- A batch loop stops at the first context cancellation instead of reporting one failure per remaining item.
- A download writes to a temporary file in the target directory and renames it on success, so a failure leaves no partial file. It refuses to overwrite without `--force`.
- An upload never sends an absolute path or one that climbs out with `..`: it sends the base name, because a full path exposes the user's directory layout and OS account name. A relative path such as `briefs/q4.pdf` is kept.
- Not-found errors name the resource and the next step (jobs are kept 24 hours; `asset list` shows valid ids).

## 15. Shared helpers, one copy **Search before adding a helper; parallel work duplicates them.**

- Command helpers used by more than one command live in `src/pkg/cli/common.go`; test helpers used by more than one test file (`runCLI`, `stubEditor`, `fastPoll`) live in `src/pkg/cli/cli_test.go`.
- Ignore a write's result explicitly (`_, _ = fmt.Fprintf(...)`) and close in a closure (`defer func() { _ = f.Close() }()`); golangci-lint's errcheck fails CI otherwise.
- Compare against constants (`outputJSON`), not string literals.

## 16. Smoke test against staging before a PR **Unit tests prove the code matches our reading of the spec; only the server proves the reading.**

- Run every changed command once against `https://stg-api.persado.com/api` with `--api-key-file`. Never print, log or commit the key. Keep staging assets few and small.

## Pre-Commit Checklist

- [ ] Patch version incremented in `src/pkg/version/version.go` for this landing on `develop` (Rule 8)
- [ ] `CHANGELOG.md` updated under this version's section for any user-facing change (Rule 6)
- [ ] All Must-Follow-Rules (MFR) in `doc/MEMORY.md` reviewed
- [ ] No code violates an existing MFR
- [ ] New MFR created/amended if a bug was fixed
- [ ] `doc/MEMORY.md` updated with session log entry
- [ ] `doc/MEMORY.md` under 400 lines (prune if over)
- [ ] `make test-race` passes
- [ ] The pre-push hook (`make ci`) passes on `git push`
- [ ] Coverage ≥ 80 % on changed packages
- [ ] Every modified/created source file has a paired `_test.go` companion (Rule 5)
- [ ] `make vuln` passes: govulncheck reports no reachable vulnerability (Rule 10)
- [ ] Every created or modified diagram has its `doc/diagrams/<name>.mmd` rendered to `<name>.png` with `mmdc` (Rule 7)

## Further Reading

`doc/` holds the Markdown documents below, and this section is the index to them. Diagrams live in `doc/diagrams/`,
and superseded documents in `doc/archive/`. The documents come in two kinds, and the distinction matters when you use
one as evidence:

- **Living reference**: describes the system as it is now. If it disagrees with `src/`, the doc is wrong and should be
  fixed.
- **Dated analysis / plan**: a point-in-time record with a code baseline and a decision log. Its findings are history:
  read it for *why* a choice was made, never as a statement about today's tree. Verify any `file:line` in one against
  the current source before acting on it.

### Living reference

| Document | Contents |
|---|---|
| [README.md](README.md) | What the project is, prerequisites, quickstart. |
| [USER_MANUAL.md](USER_MANUAL.md) | Task-by-task guide to every command, with screenshots in `doc/manual/`. |
| [CHANGELOG.md](CHANGELOG.md) | Every user-facing change, by version (Rule 6). |
| [doc/architecture.md](doc/architecture.md) | Source layout, run-time command flow, build stamping, how to add a command. |
| [doc/MEMORY.md](doc/MEMORY.md) | Must-Follow Rules and the session log. |

### Dated analysis / plan

None yet. Open work lists are the root `REMAINING_WORK_<ISODate>.md` files (Rule 9).

### Diagrams

Every Mermaid source is `doc/diagrams/<name>.mmd`, with its rendered `doc/diagrams/<name>.png` beside it. A document
embeds the PNG and links the `.mmd`. Rule 7 gives the render command.

| Source | Embedded in |
|---|---|
| [doc/diagrams/architecture.mmd](doc/diagrams/architecture.mmd) | [doc/architecture.md](doc/architecture.md) |
| [doc/diagrams/command-flow.mmd](doc/diagrams/command-flow.mmd) | [doc/architecture.md](doc/architecture.md) |
| [doc/diagrams/edit-flow.mmd](doc/diagrams/edit-flow.mmd) | [doc/architecture.md](doc/architecture.md) |
