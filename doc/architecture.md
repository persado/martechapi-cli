# Architecture

`persadomapicli` is a single static Go binary. `main` hands its arguments to `cli.Run`, which builds a cobra command tree, checks GitHub for a newer release alongside the command, and returns the process exit code. The build stamps the commit and build time into the `version` package.

![Build-time stamping, the run-time command flow and the background update check](diagrams/architecture.png)

<sub>Diagram source: [diagrams/architecture.mmd](diagrams/architecture.mmd)</sub>

## Source layout

| Path | Contents |
|---|---|
| `src/cmd/persadomapicli/main.go` | Entry point. Calls `cli.Run` with `os.Args[1:]`, stdout and stderr, then exits with the returned code. |
| `src/pkg/cli/root.go` | Root cobra command and `Run`. `Run` prints a returned error as `error: <message>` on stderr and returns exit code 1. The root's `PersistentPreRunE` prints the API banner for commands annotated `annotationAPI`. |
| `src/pkg/cli/version.go` | `version` command. |
| `src/pkg/version/version.go` | `const Version` (Rule 8), `const Product`, the `Commit` and `BuildTime` stamps, and the `String` (full), `Short` and `Banner` formats. |
| `src/pkg/update/update.go` | `Latest` (latest release tag from GitHub), `Newer` (SemVer comparison) and the install one-liners. |

## Build stamping

`make build` passes `-ldflags "-X .../version.Commit=<short HEAD> -X .../version.BuildTime=<UTC RFC 3339>"`. A binary built without the Makefile (`go build`, `go install`) reports `unknown` for both stamps. `Version` is a constant and is never stamped: it changes only through a commit to `version.go`.

## Update check

`cli.Run` starts `update.Latest` in a goroutine before it executes the command. `Latest` sends a `HEAD` request to `https://github.com/persado/martechapi-cli/releases/latest` without following the redirect, and takes the version from the `vX.Y.Z` tag at the end of the `Location` header. After the command returns, `Run` waits for the result for at most `updateWait` (2 s), then cancels the request. If `update.Newer` reports a higher version, `Run` prints the notice on stderr with the one-liner for the current OS: `install.ps1` on Windows, `install.sh` elsewhere. Any error, including no network, prints nothing.

## Release builds

`make dist` cross-compiles the same `main` package with `CGO_ENABLED=0` for `windows/amd64`, `windows/arm64`, `linux/amd64` and `darwin/arm64` (the `PLATFORMS` variable), names each binary `persadomapicli-<os>-<arch>` with `darwin` shown as `macos` and `.exe` on Windows, and writes `SHA256SUMS`. The `release` workflow runs it on every push to `main` and attaches the output to release `vX.Y.Z`.

## Adding a command

1. Create `src/pkg/cli/<command>.go` with a `new<Command>Cmd() *cobra.Command` constructor, and its companion `<command>_test.go` (Rule 5).
2. Register it in `newRootCmd` with `root.AddCommand`.
3. Write output to `cmd.OutOrStdout()` and return errors from `RunE`, so tests can drive the command through `Run` with buffers.
