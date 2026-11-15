// Package version holds the application version and the build stamps.
package version

import "fmt"

// Version is the application version (SemVer 2.0.0). AGENTS.md Rule 8 governs
// when it changes: every landing on develop increments PATCH.
const Version = "0.1.2"

// Product is the name printed before the version.
const Product = "Persado Martech API"

// Commit and BuildTime are stamped by `make build` through -ldflags -X.
var (
	Commit    = "unknown"
	BuildTime = "unknown"
)

// String returns the full version: "0.1.2 (commit fed3a10, built 2026-11-15T09:53:19Z)".
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildTime)
}

// Short returns "0.1.2-fed3a10", or the bare Version when no commit is stamped.
func Short() string {
	if Commit == "unknown" {
		return Version
	}
	return Version + "-" + Commit
}

// Banner returns the line printed before an API command runs: "Persado Martech API 0.1.2-fed3a10".
func Banner() string {
	return Product + " " + Short()
}
