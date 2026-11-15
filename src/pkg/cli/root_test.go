package cli

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/persado/martechapi-cli/src/pkg/update"
	"github.com/persado/martechapi-cli/src/pkg/version"
	"github.com/spf13/cobra"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "help lists version command", args: []string{"--help"}, wantCode: 0, wantStdout: "version"},
		{name: "unknown command fails", args: []string{"bogus"}, wantCode: 1, wantStderr: `error: unknown command "bogus"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tc.args, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr: %q)", code, tc.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tc.wantStdout)
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}

func stubUpdateCheck(t *testing.T, fn func(context.Context) (string, error)) {
	t.Helper()
	old := updateCheck
	t.Cleanup(func() { updateCheck = old })
	updateCheck = fn
}

func TestRunUpdateNotice(t *testing.T) {
	tests := []struct {
		name       string
		check      func(context.Context) (string, error)
		wantNotice bool
	}{
		{"newer release", func(context.Context) (string, error) { return "99.0.0", nil }, true},
		{"same release", func(context.Context) (string, error) { return version.Version, nil }, false},
		{"check fails", func(context.Context) (string, error) { return "", errors.New("offline") }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubUpdateCheck(t, tc.check)
			var stdout, stderr bytes.Buffer
			if code := Run(context.Background(), []string{"version"}, &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			got := stderr.String()
			if !tc.wantNotice {
				if got != "" {
					t.Fatalf("stderr = %q, want empty", got)
				}
				return
			}
			for _, want := range []string{"New version found: 99.0.0", "installed: " + version.Version, update.InstallCommand(runtime.GOOS)} {
				if !strings.Contains(got, want) {
					t.Errorf("stderr %q does not contain %q", got, want)
				}
			}
		})
	}
}

func TestRunDoesNotWaitPastUpdateWait(t *testing.T) {
	stubUpdateCheck(t, func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	old := updateWait
	t.Cleanup(func() { updateWait = old })
	updateWait = 50 * time.Millisecond

	var stdout, stderr bytes.Buffer
	start := time.Now()
	if code := Run(context.Background(), []string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Run took %v with a hung update check, want about %v", elapsed, updateWait)
	}
}

func TestAPIBanner(t *testing.T) {
	tests := []struct {
		name       string
		annotate   bool
		wantBanner bool
	}{
		{"API command prints banner", true, true},
		{"other command prints nothing", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sub := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
			if tc.annotate {
				sub.Annotations = map[string]string{annotationAPI: ""}
			}
			root := newRootCmd()
			root.AddCommand(sub)
			var stdout, stderr bytes.Buffer
			root.SetArgs([]string{"probe"})
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			want := ""
			if tc.wantBanner {
				want = version.Banner() + "\n"
			}
			if stderr.String() != want {
				t.Fatalf("stderr = %q, want %q", stderr.String(), want)
			}
		})
	}
}
