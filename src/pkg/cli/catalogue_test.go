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

const catalogueBody = `{"defaultChannel":"EMAIL_SUBJECT",
"channels":[
 {"key":"EMAIL_SUBJECT","label":"Email subject","touchpoints":[{"key":"text","label":"Text"}]},
 {"key":"PUSH_NOTIFICATION","label":"Push notification","touchpoints":[{"key":"headline","label":""},{"key":"body","label":"Body"}]}],
"displayChannels":[{"key":"EMAIL","label":"Email"}],
"industries":[{"key":"FINANCIAL_SERVICES","label":"Financial services"}],
"limits":{"maxBodyBytes":1048576,"maxTouchpointLength":0,"maxScoreVariants":10,"maxWaitSeconds":900}}`

func TestCatalogueCmd(t *testing.T) {
	const fullText = `Channels (default EMAIL_SUBJECT)
Key Label Touchpoints
EMAIL_SUBJECT Email subject text (Text)
PUSH_NOTIFICATION Push notification headline, body (Body)

Display channels
Key Label
EMAIL Email

Industries
Key Label
FINANCIAL_SERVICES Financial services

Limits
Limit Value
Max body bytes 1048576
Max touchpoint length -
Max score variants 10
Max wait (seconds) 900`

	tests := []struct {
		name     string
		args     []string
		status   int
		body     string
		noKey    bool
		wantCode int
		wantOut  string
		wantErr  string
		wantHits int32
	}{
		{name: "all sections", body: catalogueBody, wantCode: 0, wantOut: fullText, wantHits: 1},
		{name: "one section", args: []string{"--section", "channels"}, body: catalogueBody,
			wantOut:  "Channels (default EMAIL_SUBJECT)\nKey Label Touchpoints\nEMAIL_SUBJECT Email subject text (Text)\nPUSH_NOTIFICATION Push notification headline, body (Body)",
			wantHits: 1},
		{name: "display channels section", args: []string{"--section", "display-channels"}, body: catalogueBody,
			wantOut: "Display channels\nKey Label\nEMAIL Email", wantHits: 1},
		{name: "industries section", args: []string{"--section", "industries"}, body: catalogueBody,
			wantOut: "Industries\nKey Label\nFINANCIAL_SERVICES Financial services", wantHits: 1},
		{name: "limits section", args: []string{"--section", "limits"}, body: catalogueBody,
			wantOut: "Limits\nLimit Value\nMax body bytes 1048576\nMax touchpoint length -\nMax score variants 10\nMax wait (seconds) 900", wantHits: 1},
		{name: "json prints the raw document", args: []string{"-o", "json"}, body: catalogueBody,
			wantOut: "", wantHits: 1},
		{name: "unknown section is rejected before the request", args: []string{"--section", "bogus"}, body: catalogueBody,
			wantCode: 1, wantErr: `error: --section must be one of channels, display-channels, industries, limits, not "bogus"`, wantHits: 0},
		{name: "unknown output is rejected", args: []string{"-o", "yaml"}, body: catalogueBody,
			wantCode: 1, wantErr: "--output must be text or json", wantHits: 0},
		{name: "server error", status: http.StatusInternalServerError,
			body:     `{"error":{"code":"internal","message":"boom","requestId":"req-1"}}`,
			wantCode: 1, wantErr: "error: boom (HTTP 500, internal)", wantHits: 1},
		{name: "no API key", noKey: true, body: catalogueBody,
			wantCode: 1, wantErr: "no API key", wantHits: 0},
		{name: "positional argument is rejected", args: []string{"extra"}, body: catalogueBody,
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

			args := append([]string{"catalogue"}, tc.args...)
			args = append(args, "--base-url", srv.URL)
			if !tc.noKey {
				args = append(args, "--api-key", "pmk_test")
			}
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
			if strings.Contains(strings.Join(tc.args, " "), "json") {
				if !strings.Contains(stdout.String(), `"defaultChannel": "EMAIL_SUBJECT"`) {
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

func TestCatalogueCmdRequest(t *testing.T) {
	t.Setenv(envAPIKey, "")
	t.Setenv(envBaseURL, "")
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())
		_, _ = io.WriteString(w, catalogueBody)
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	args := []string{"catalogue", "--base-url", srv.URL, "--api-key", "pmk_test", "--lang", "fr"}
	if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	if got.Method != http.MethodGet || got.URL.Path != "/v1/catalogue" {
		t.Fatalf("request = %s %s, want GET /v1/catalogue", got.Method, got.URL.Path)
	}
	if h := got.Header.Get("Authorization"); h != "Bearer pmk_test" {
		t.Fatalf("Authorization = %q, want Bearer pmk_test", h)
	}
	if h := got.Header.Get("Accept-Language"); h != "fr" {
		t.Fatalf("Accept-Language = %q, want fr", h)
	}
	if got.ContentLength != 0 {
		t.Fatalf("request has a body, want none")
	}
	if !strings.HasPrefix(stderr.String(), "Persado Martech API") {
		t.Fatalf("stderr = %q, want the banner first", stderr.String())
	}
}
