package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/version"
)

func TestVersionCmd(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	if want := version.Product + " " + version.String() + "\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestVersionCmdRejectsArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"version", "extra"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
