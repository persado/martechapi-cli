// Package version holds the application version and the build stamps.
package version

import "fmt"

// Version is the application version (SemVer 2.0.0). AGENTS.md Rule 8 governs
// when it changes: every landing on develop increments PATCH.
const Version = "0.1.1"

// Commit and BuildTime are stamped by `make build` through -ldflags -X.
var (
	Commit    = "unknown"
	BuildTime = "unknown"
)

// String returns the line printed by `persadomapicli version`.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildTime)
}
