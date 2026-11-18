package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/persado/martechapi-cli/src/pkg/editor"
)

// TestMain stubs the update check for the whole package: tests must not reach the network.
func TestMain(m *testing.M) {
	updateCheck = func(context.Context) (string, error) { return "", nil }
	os.Exit(m.Run())
}

// runCLI runs the CLI and returns the exit code, stdout and stderr.
func runCLI(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// stubEditor replaces runEditor for one test and restores it on cleanup.
func stubEditor(t *testing.T, fn func(ctx context.Context, title, text string, v editor.Validator) (string, error)) {
	t.Helper()
	old := runEditor
	t.Cleanup(func() { runEditor = old })
	runEditor = fn
}

// fastPoll sets pollInterval to 1 ms for the test.
func fastPoll(t *testing.T) {
	t.Helper()
	old := pollInterval
	t.Cleanup(func() { pollInterval = old })
	pollInterval = time.Millisecond
}

// normalizeLines collapses runs of spaces so tests do not depend on tabwriter padding.
func normalizeLines(s string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		out = append(out, strings.Join(strings.Fields(line), " "))
	}
	return strings.Join(out, "\n")
}
