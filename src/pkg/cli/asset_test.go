package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNewAssetCmd(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"asset", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d (stderr: %q)", code, stderr.String())
	}
	for _, sub := range []string{"create", "list", "get", "download"} {
		if !strings.Contains(stdout.String(), sub) {
			t.Errorf("asset help does not list %q:\n%s", sub, stdout.String())
		}
	}
}
