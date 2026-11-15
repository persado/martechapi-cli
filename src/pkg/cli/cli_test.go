package cli

import (
	"context"
	"os"
	"testing"
)

// TestMain stubs the update check for the whole package: tests must not reach the network.
func TestMain(m *testing.M) {
	updateCheck = func(context.Context) (string, error) { return "", nil }
	os.Exit(m.Run())
}
