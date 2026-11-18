package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const serverVersionBody = `{"version":"0.80.74","commit":"a1b2c3","buildTime":"2026-11-17T09:30:00Z",
"specDate":"2026-11-17","environment":"prod-eks01"}`

func TestServerVersionCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		status   int
		body     string
		wantCode int
		wantOut  string
		wantErr  string
		wantHits int32
	}{
		{name: "text lines", body: serverVersionBody,
			wantOut:  "Version 0.80.74\nCommit a1b2c3\nBuild time 2026-11-17T09:30:00Z\nSpec date 2026-11-17\nEnvironment prod-eks01",
			wantHits: 1},
		{name: "json prints the raw document", args: []string{"-o", "json"}, body: serverVersionBody, wantHits: 1},
		{name: "empty values show a dash", body: `{"version":"","commit":"unknown","buildTime":"unknown","specDate":"","environment":"dev"}`,
			wantOut: "Version -\nCommit unknown\nBuild time unknown\nSpec date -\nEnvironment dev", wantHits: 1},
		{name: "server error", status: http.StatusInternalServerError,
			body:     `{"error":{"code":"internal","message":"boom","requestId":"req-2"}}`,
			wantCode: 1, wantErr: "error: boom (HTTP 500, internal)", wantHits: 1},
		{name: "unknown output is rejected", args: []string{"-o", "yaml"}, body: serverVersionBody,
			wantCode: 1, wantErr: "--output must be text or json", wantHits: 0},
		{name: "positional argument is rejected", args: []string{"extra"}, body: serverVersionBody,
			wantCode: 1, wantErr: "unknown command", wantHits: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envAPIKey, "")
			t.Setenv(envBaseURL, "")
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			args := append([]string{"server-version"}, tc.args...)
			args = append(args, "--base-url", srv.URL)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr: %q)", code, tc.wantCode, stderr.String())
			}
			if tc.wantErr != "" && !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), tc.wantErr)
			}
			if got := hits.Load(); got != tc.wantHits {
				t.Fatalf("server requests = %d, want %d", got, tc.wantHits)
			}
			if tc.wantCode != 0 {
				return
			}
			if len(tc.args) > 0 && tc.args[0] == "-o" {
				if !strings.Contains(stdout.String(), `"specDate": "2026-11-17"`) {
					t.Fatalf("json stdout = %q, want the indented raw document", stdout.String())
				}
				return
			}
			if got := normalizeLines(stdout.String()); got != tc.wantOut {
				t.Fatalf("stdout =\n%s\nwant\n%s", got, tc.wantOut)
			}
		})
	}
}

func TestServerVersionCmdNeedsNoKey(t *testing.T) {
	t.Setenv(envAPIKey, "pmk_env_key")
	t.Setenv(envBaseURL, "")
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())
		_, _ = io.WriteString(w, serverVersionBody)
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	args := []string{"server-version", "--base-url", srv.URL, "--api-key", "pmk_flag_key"}
	if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	if got.Method != http.MethodGet || got.URL.Path != "/version" {
		t.Fatalf("request = %s %s, want GET /version", got.Method, got.URL.Path)
	}
	if h := got.Header.Get("Authorization"); h != "" {
		t.Fatalf("Authorization = %q, want none on /version", h)
	}
}
