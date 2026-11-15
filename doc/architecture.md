# Architecture

`persadomapicli` is a single static Go binary. `main` hands its arguments to `cli.Run`, which builds a cobra command tree and returns the process exit code. The build stamps the commit and build time into the `version` package.

![Build-time stamping and run-time command flow](diagrams/architecture.png)

<sub>Diagram source: [diagrams/architecture.mmd](diagrams/architecture.mmd)</sub>

## Source layout

| Path | Contents |
|---|---|
| `src/cmd/persadomapicli/main.go` | Entry point. Calls `cli.Run` with `os.Args[1:]`, stdout and stderr, then exits with the returned code. |
| `src/pkg/cli/root.go` | Root cobra command and `Run`. `Run` prints a returned error as `error: <message>` on stderr and returns exit code 1. |
| `src/pkg/cli/version.go` | `version` command. |
| `src/pkg/version/version.go` | `const Version` (Rule 8) and the `Commit` and `BuildTime` stamps. |

## Build stamping

`make build` passes `-ldflags "-X .../version.Commit=<short HEAD> -X .../version.BuildTime=<UTC RFC 3339>"`. A binary built without the Makefile (`go build`, `go install`) reports `unknown` for both stamps. `Version` is a constant and is never stamped: it changes only through a commit to `version.go`.

## Release builds

`make dist` cross-compiles the same `main` package with `CGO_ENABLED=0` for `windows/amd64`, `windows/arm64`, `linux/amd64` and `darwin/arm64` (the `PLATFORMS` variable), names each binary `persadomapicli-<os>-<arch>` with `darwin` shown as `macos` and `.exe` on Windows, and writes `SHA256SUMS`. The `release` workflow runs it on every push to `main` and attaches the output to release `vX.Y.Z`.

## Adding a command

1. Create `src/pkg/cli/<command>.go` with a `new<Command>Cmd() *cobra.Command` constructor, and its companion `<command>_test.go` (Rule 5).
2. Register it in `newRootCmd` with `root.AddCommand`.
3. Write output to `cmd.OutOrStdout()` and return errors from `RunE`, so tests can drive the command through `Run` with buffers.
