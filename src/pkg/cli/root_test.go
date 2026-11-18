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

func TestRootHelpListsEveryCommand(t *testing.T) {
	code, stdout, stderr := runCLI("--help")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, name := range []string{"version", "server-version", "catalogue", "score", "optimize", "job", "performance", "uplift", "asset"} {
		if !strings.Contains(stdout, name) {
			t.Errorf("help does not list %q:\n%s", name, stdout)
		}
	}
	if stderr != "" {
		t.Errorf("help must not print the banner: %q", stderr)
	}
}

func TestRootGlobalFlags(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"api-key", "api-key-file", "base-url", "lang", "output"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing persistent flag --%s", name)
		}
	}
	if f := root.PersistentFlags().ShorthandLookup("o"); f == nil || f.Name != "output" || f.DefValue != "text" {
		t.Errorf("-o = %+v", f)
	}
	// Every subcommand inherits them.
	for _, sub := range root.Commands() {
		if sub.InheritedFlags().Lookup("output") == nil {
			t.Errorf("%s does not inherit --output", sub.Name())
		}
	}
}

func TestRootCommandsMarkedAsAPI(t *testing.T) {
	apiCmds := map[string]bool{"server-version": true, "catalogue": true, "score": true, "optimize": true, "job": true, "performance": true, "uplift": true, "asset": true}
	for _, sub := range newRootCmd().Commands() {
		name := sub.Name()
		if name == "help" || name == "completion" {
			continue
		}
		_, marked := sub.Annotations[annotationAPI]
		if name == "asset" {
			// asset is a group: its subcommands carry the annotation.
			marked = true
			for _, c := range sub.Commands() {
				if _, ok := c.Annotations[annotationAPI]; !ok {
					t.Errorf("asset %s is not marked as an API command", c.Name())
				}
			}
		}
		if marked != apiCmds[name] {
			t.Errorf("%s: API-marked = %v, want %v", name, marked, apiCmds[name])
		}
	}
}

func TestRootOutputValidation(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
		wantBanner bool
	}{
		{"api command with bad output fails before the banner", []string{"job", testJobID, "-o", "yaml"}, 1, `--output must be text or json, not "yaml"`, false},
		{"non-API command ignores --output", []string{"version", "-o", "yaml"}, 0, "", false},
		{"api command with good output prints the banner", []string{"job", "bad-id", "-o", "json"}, 1, "invalid request", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := runCLI(tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, tc.wantCode, stderr)
			}
			if !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr %q lacks %q", stderr, tc.wantStderr)
			}
			if got := strings.Contains(stderr, version.Banner()); got != tc.wantBanner {
				t.Errorf("banner present = %v, want %v (stderr %q)", got, tc.wantBanner, stderr)
			}
		})
	}
}

func TestRootBannerOnStderrOnly(t *testing.T) {
	srv, _ := ctJobServer(t, ctJob("completed", ""))
	code, stdout, stderr := runCLI("job", testJobID, "--base-url", srv.URL, "--api-key", "pmk_test")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.HasPrefix(stderr, version.Banner()+"\n") {
		t.Errorf("stderr = %q, want banner first", stderr)
	}
	if strings.Contains(stdout, version.Banner()) {
		t.Errorf("stdout must not carry the banner: %q", stdout)
	}
}

func TestRootVersionHasNoBanner(t *testing.T) {
	code, _, stderr := runCLI("version")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}
