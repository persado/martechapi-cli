package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/persado/martechapi-cli/src/pkg/editor"
)

func ctScoreArgs(srv string, args ...string) []string {
	return append(append([]string{"score"}, args...), "--base-url", srv, "--api-key", "pmk_test")
}

func ctBody(t *testing.T, r seenRequest) api.ScoreRequest {
	t.Helper()
	var req api.ScoreRequest
	if err := json.Unmarshal([]byte(r.Body), &req); err != nil {
		t.Fatalf("request body %q: %v", r.Body, err)
	}
	return req
}

func TestScoreCommand(t *testing.T) {
	fastPoll(t)
	completed := ctJob("completed", ctCompletedResult)
	dir := t.TempDir()
	file := filepath.Join(dir, "req.json")
	if err := os.WriteFile(file, []byte(`{"channel":"PUSH_NOTIFICATION","account":{"name":"FileBrand","country":"GR"},"variants":[{"headline":"H","body":"B"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	badFile := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badFile, []byte(`{"bogus":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		args      []string
		responses []string
		wantCode  int
		wantOut   []string
		wantErr   []string
		wantReqs  int
		check     func(t *testing.T, seen []seenRequest)
	}{
		{
			name:      "flags only, default waits 50 s",
			args:      []string{"-a", "Acme", "--variant", "Hello", "-v", "World"},
			responses: []string{completed},
			wantOut:   []string{testJobID, "completed", "Variants"},
			wantErr:   []string{"Persado Martech API"},
			wantReqs:  1,
			check: func(t *testing.T, seen []seenRequest) {
				r := seen[0]
				if r.Method != http.MethodPost || r.Path != "/v1/score" || r.RawQuery != "wait=50" || r.Auth != "Bearer pmk_test" {
					t.Errorf("request = %+v", r)
				}
				got := ctBody(t, r)
				if got.Account.Name != "Acme" || len(got.Variants) != 2 || got.Variants[0]["text"] != "Hello" || got.Variants[1]["text"] != "World" {
					t.Errorf("body = %+v", got)
				}
			},
		},
		{
			name:      "unknown industry is rejected locally",
			args:      []string{"-a", "Acme", "-c", "sms_ems", "--country", "gr", "--language", "EL", "--industry", "retail", "-v", "Hi"},
			responses: []string{completed},
			wantCode:  1,
			wantErr:   []string{"account.industry"},
			wantReqs:  0,
		},
		{
			name:      "json touchpoint variant",
			args:      []string{"-a", "Acme", "-c", "push_notification", "-v", `{"headline":"Hi","body":"Apply"}`},
			responses: []string{completed},
			wantReqs:  1,
			check: func(t *testing.T, seen []seenRequest) {
				got := ctBody(t, seen[0])
				if got.Channel != "PUSH_NOTIFICATION" || got.Variants[0]["headline"] != "Hi" || got.Variants[0]["body"] != "Apply" {
					t.Errorf("body = %+v", got)
				}
			},
		},
		{
			name:      "async sends no wait query and prints the job without polling",
			args:      []string{"-a", "Acme", "-v", "Hi", "--async"},
			responses: []string{ctJob("queued", "")},
			wantOut:   []string{testJobID, "queued"},
			wantReqs:  1,
			check: func(t *testing.T, seen []seenRequest) {
				if seen[0].RawQuery != "" {
					t.Errorf("query = %q, want none", seen[0].RawQuery)
				}
			},
		},
		{
			name:      "polls until completed",
			args:      []string{"-a", "Acme", "-v", "Hi"},
			responses: []string{ctJob("queued", ""), ctJob("running", ""), completed},
			wantOut:   []string{"completed", "Variants"},
			wantErr:   []string{"Waiting for score " + testJobID},
			wantReqs:  3,
			check: func(t *testing.T, seen []seenRequest) {
				for _, r := range seen[1:] {
					if r.Method != http.MethodGet || r.Path != "/v1/jobs/"+testJobID {
						t.Errorf("poll request = %+v", r)
					}
				}
			},
		},
		{
			name:      "failed job exits 1",
			args:      []string{"-a", "Acme", "-v", "Hi"},
			responses: []string{ctJob("failed", `"error":{"code":"pii","message":"personal data"}`)},
			wantCode:  1,
			wantOut:   []string{"pii: personal data"},
			wantErr:   []string{"error: job failed"},
			wantReqs:  1,
		},
		{
			name:      "json output is raw",
			args:      []string{"-a", "Acme", "-v", "Hi", "-o", "json"},
			responses: []string{ctJob("completed", `"extra":1`)},
			wantOut:   []string{`"extra": 1`},
			wantReqs:  1,
		},
		{
			name:      "file only",
			args:      []string{"--file", file},
			responses: []string{completed},
			wantReqs:  1,
			check: func(t *testing.T, seen []seenRequest) {
				got := ctBody(t, seen[0])
				if got.Channel != "PUSH_NOTIFICATION" || got.Account.Name != "FileBrand" || got.Account.Country != "GR" || got.Variants[0]["headline"] != "H" {
					t.Errorf("body = %+v", got)
				}
			},
		},
		{
			name:      "file plus overrides: only changed flags win",
			args:      []string{"-f", file, "-a", "Override", "--language", "EL", "--variant", `{"headline":"New","body":"Text"}`},
			responses: []string{completed},
			wantReqs:  1,
			check: func(t *testing.T, seen []seenRequest) {
				got := ctBody(t, seen[0])
				if got.Account.Name != "Override" || got.Account.Language != "el" {
					t.Errorf("account = %+v", got.Account)
				}
				if got.Account.Country != "GR" || got.Channel != "PUSH_NOTIFICATION" {
					t.Errorf("unchanged file values lost: %+v", got)
				}
				if len(got.Variants) != 1 || got.Variants[0]["headline"] != "New" {
					t.Errorf("variants = %+v", got.Variants)
				}
			},
		},
		{
			name:      "file with unknown field",
			args:      []string{"-f", badFile},
			responses: []string{completed},
			wantCode:  1,
			wantErr:   []string{"unknown field"},
		},
		{
			name:      "missing file",
			args:      []string{"-f", filepath.Join(dir, "nope.json")},
			responses: []string{completed},
			wantCode:  1,
			wantErr:   []string{"read request"},
		},
		{
			name:      "bad json variant",
			args:      []string{"-a", "Acme", "-v", `{"headline":`},
			responses: []string{completed},
			wantCode:  1,
			wantErr:   []string{"--variant"},
		},
		{
			name:      "validation error before any request",
			args:      []string{"-v", "Hi"},
			responses: []string{completed},
			wantCode:  1,
			wantErr:   []string{"invalid request (use --edit to fix it interactively)", "account.name"},
		},
		{
			name:      "too many variants before any request",
			args:      []string{"-a", "Acme", "-v", "1", "-v", "2", "-v", "3", "-v", "4", "-v", "5"},
			responses: []string{completed},
			wantCode:  1,
			wantErr:   []string{"variants"},
		},
		{
			name:      "no variants",
			args:      []string{"-a", "Acme"},
			responses: []string{completed},
			wantCode:  1,
			wantErr:   []string{"variants"},
		},
		{
			name: "api 400 with details on stderr",
			args: []string{"-a", "Acme", "-v", "Hi"},
			responses: []string{`HTTP 400 {"error":{"code":"invalid_request","message":"bad request","requestId":"req-9",` +
				`"details":[{"field":"variants[0].text","reason":"too long"}],"categories":["email"]}}`},
			wantCode: 1,
			wantErr:  []string{"error: bad request (HTTP 400, invalid_request)", "variants[0].text: too long", "personal data found: email", "request id: req-9"},
			wantReqs: 1,
		},
		{
			name:      "positional args rejected",
			args:      []string{"extra"},
			responses: []string{completed},
			wantCode:  1,
			wantErr:   []string{"unknown command"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envAPIKey, "")
			t.Setenv(envBaseURL, "")
			srv, seen := ctJobServer(t, tc.responses...)
			code, stdout, stderr := runCLI(ctScoreArgs(srv.URL, tc.args...)...)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", code, tc.wantCode, stdout, stderr)
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(stdout, w) {
					t.Errorf("stdout lacks %q:\n%s", w, stdout)
				}
			}
			for _, w := range tc.wantErr {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr lacks %q:\n%s", w, stderr)
				}
			}
			if len(*seen) != tc.wantReqs {
				t.Errorf("server saw %d requests, want %d", len(*seen), tc.wantReqs)
			}
			if tc.check != nil && len(*seen) > 0 {
				tc.check(t, *seen)
			}
		})
	}
}

func TestScoreCommandEnumNormalisation(t *testing.T) {
	srv, seen := ctJobServer(t, ctJob("completed", ""))
	code, _, stderr := runCLI(ctScoreArgs(srv.URL, "-a", "Acme", "-c", "sms_ems", "--display-channel", "billboard",
		"--country", "gr", "--language", "EL", "--region", "R", "--parent-id", "9", "-v", "Hi")...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := ctBody(t, (*seen)[0])
	want := api.Account{Name: "Acme", Country: "GR", Region: "R", Language: "el", ParentID: 9}
	if got.Channel != "SMS_EMS" || got.DisplayChannel != "BILLBOARD" || got.Account != want {
		t.Errorf("body = %+v", got)
	}
}

func TestScoreCommandStdinFile(t *testing.T) {
	// Run has no stdin parameter; drive the command directly.
	srv, seen := ctJobServer(t, ctJob("completed", ""))
	cmd := newScoreCmd(&globalFlags{output: outputText, baseURL: srv.URL, apiKey: "pmk_test"})
	cmd.SetIn(strings.NewReader(`{"account":{"name":"Stdin"},"variants":[{"text":"Hi"}]}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--file", "-"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := ctBody(t, (*seen)[0]); got.Account.Name != "Stdin" {
		t.Errorf("body = %+v", got)
	}
}

func TestScoreCommandEdit(t *testing.T) {
	fastPoll(t)
	const edited = `{"channel":"EMAIL_SUBJECT","account":{"name":"Edited"},"variants":[{"text":"Edited copy"}]}`
	tests := []struct {
		name     string
		args     []string
		editor   func(t *testing.T) func(ctx context.Context, title, text string, v editor.Validator) (string, error)
		wantCode int
		wantErr  string
		wantReqs int
		check    func(t *testing.T, seen []seenRequest)
	}{
		{
			name: "template is prefilled from flags, edit succeeds",
			args: []string{"-a", "Flagged", "-c", "sms_ems", "-v", "From flag"},
			editor: func(t *testing.T) func(context.Context, string, string, editor.Validator) (string, error) {
				return func(_ context.Context, title, text string, v editor.Validator) (string, error) {
					if title != "Score request (POST /v1/score)" {
						t.Errorf("title = %q", title)
					}
					for _, w := range []string{"// Score 1 to 4 variants", "// channel: EMAIL_SUBJECT", "// Touchpoint keys per channel",
						`"channel": "SMS_EMS"`, `"name": "Flagged"`, `"text": "From flag"`} {
						if !strings.Contains(text, w) {
							t.Errorf("template lacks %q:\n%s", w, text)
						}
					}
					// keys appear in a stable, documented order
					if strings.Index(text, `"channel"`) > strings.Index(text, `"account"`) || strings.Index(text, `"account"`) > strings.Index(text, `"variants"`) {
						t.Errorf("key order wrong:\n%s", text)
					}
					return edited, v(edited)
				}
			},
			wantReqs: 1,
			check: func(t *testing.T, seen []seenRequest) {
				if got := ctBody(t, seen[0]); got.Account.Name != "Edited" || got.Variants[0]["text"] != "Edited copy" {
					t.Errorf("body = %+v", got)
				}
			},
		},
		{
			name: "empty template defaults channel and one empty variant",
			editor: func(t *testing.T) func(context.Context, string, string, editor.Validator) (string, error) {
				return func(_ context.Context, _, text string, v editor.Validator) (string, error) {
					for _, w := range []string{`"channel": "EMAIL_SUBJECT"`, `"displayChannel": ""`, `"text": ""`} {
						if !strings.Contains(text, w) {
							t.Errorf("template lacks %q:\n%s", w, text)
						}
					}
					return edited, v(edited)
				}
			},
			wantReqs: 1,
		},
		{
			name: "parent id shown when set",
			args: []string{"-a", "A", "--parent-id", "5", "-v", "x"},
			editor: func(t *testing.T) func(context.Context, string, string, editor.Validator) (string, error) {
				return func(_ context.Context, _, text string, v editor.Validator) (string, error) {
					if !strings.Contains(text, `"parentId": 5`) {
						t.Errorf("template lacks parentId:\n%s", text)
					}
					return edited, nil
				}
			},
			wantReqs: 1,
		},
		{
			name: "validator reports a validation error",
			editor: func(t *testing.T) func(context.Context, string, string, editor.Validator) (string, error) {
				return func(_ context.Context, _, _ string, v editor.Validator) (string, error) {
					bad := `{"account":{"name":""},"variants":[]}`
					err := v(bad)
					if err == nil || !strings.Contains(err.Error(), "account.name") {
						t.Errorf("validator err = %v", err)
					}
					return bad, err
				}
			},
			wantCode: 1,
			wantErr:  "account.name",
		},
		{
			name: "canceled",
			editor: func(t *testing.T) func(context.Context, string, string, editor.Validator) (string, error) {
				return func(context.Context, string, string, editor.Validator) (string, error) {
					return "", editor.ErrCanceled
				}
			},
			wantCode: 1,
			wantErr:  "edit canceled",
		},
		{
			name: "empty text",
			editor: func(t *testing.T) func(context.Context, string, string, editor.Validator) (string, error) {
				return func(_ context.Context, _, _ string, v editor.Validator) (string, error) {
					return "", v("")
				}
			},
			wantCode: 1,
			wantErr:  "the request is empty",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubEditor(t, tc.editor(t))
			srv, seen := ctJobServer(t, ctJob("completed", ""))
			args := append([]string{"-e"}, tc.args...)
			code, _, stderr := runCLI(ctScoreArgs(srv.URL, args...)...)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %s)", code, tc.wantCode, stderr)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Errorf("stderr %q lacks %q", stderr, tc.wantErr)
			}
			if len(*seen) != tc.wantReqs {
				t.Errorf("server saw %d requests, want %d", len(*seen), tc.wantReqs)
			}
			if tc.check != nil && len(*seen) > 0 {
				tc.check(t, *seen)
			}
		})
	}
}

func TestScoreTemplate(t *testing.T) {
	got := scoreTemplate(&api.ScoreRequest{
		Channel:  "PUSH_NOTIFICATION",
		Account:  api.Account{Name: "Acme"},
		Variants: []api.Touchpoints{{"headline": "H"}, {"headline": "H2"}},
	})
	for _, w := range []string{`"channel": "PUSH_NOTIFICATION"`, `"headline": "H"`, `"headline": "H2"`, `"name": "Acme"`} {
		if !strings.Contains(got, w) {
			t.Errorf("template lacks %q:\n%s", w, got)
		}
	}
	// The template must decode back into a request once comments are removed.
	var lines []string
	for _, l := range strings.Split(got, "\n") {
		if !strings.HasPrefix(l, "//") {
			lines = append(lines, l)
		}
	}
	var back api.ScoreRequest
	if err := decodeStrict([]byte(strings.Join(lines, "\n")), &back); err != nil {
		t.Fatalf("template does not round-trip: %v", err)
	}
}

func TestScoreCommandNoAPIKey(t *testing.T) {
	t.Setenv(envAPIKey, "")
	srv, seen := ctJobServer(t, ctJob("completed", ""))
	code, _, stderr := runCLI("score", "-a", "Acme", "-v", "Hi", "--base-url", srv.URL)
	if code != 1 || !strings.Contains(stderr, "no API key") || len(*seen) != 0 {
		t.Fatalf("code %d stderr %q reqs %d", code, stderr, len(*seen))
	}
}

func TestScoreCommandKeyAndURLFromEnv(t *testing.T) {
	srv, seen := ctJobServer(t, ctJob("completed", ""))
	t.Setenv(envAPIKey, "pmk_env")
	t.Setenv(envBaseURL, srv.URL)
	code, _, stderr := runCLI("score", "-a", "Acme", "-v", "Hi")
	if code != 0 || len(*seen) != 1 || (*seen)[0].Auth != "Bearer pmk_env" {
		t.Fatalf("code %d stderr %q seen %+v", code, stderr, *seen)
	}
}
