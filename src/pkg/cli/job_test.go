package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/persado/martechapi-cli/src/pkg/editor"
)

const testJobID = "job_0123456789abcdef0123456789abcdef"

// seenRequest is what the fake server received.
type seenRequest struct {
	Method, Path, RawQuery, Auth, Body string
}

// ctJobServer serves each response in order (the last one repeats) and records requests.
// A response starting with "HTTP " is "HTTP <status> <body>".
func ctJobServer(t *testing.T, responses ...string) (*httptest.Server, *[]seenRequest) {
	t.Helper()
	var seen []seenRequest
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body bytes.Buffer
		_, _ = body.ReadFrom(r.Body)
		seen = append(seen, seenRequest{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), body.String()})
		i := int(n.Add(1)) - 1
		if i >= len(responses) {
			i = len(responses) - 1
		}
		resp, status := responses[i], http.StatusOK
		if strings.HasPrefix(resp, "HTTP ") {
			_, _ = fmt.Sscanf(resp, "HTTP %d", &status)
			resp = resp[strings.Index(resp[5:], " ")+6:]
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func ctJob(status, extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return fmt.Sprintf(`{"id":%q,"kind":"score","status":%q,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z","expiresAt":"2026-01-02T00:00:00Z"%s}`,
		testJobID, status, extra)
}

const ctCompletedResult = `"result":{"complete":true,"brand":"Acme","channel":"EMAIL_SUBJECT","variants":[` +
	`{"index":1,"text":"Hello\nworld","performance":80,"performanceBand":"HIGH","brandAlignment":4,"compliance":{"risk":"LOW","state":"done"}},` +
	`{"index":2,"text":"Second","compliance":{"state":"pending"}},{"index":3,"text":"Third"}]}`

const ctOptimizedResult = `"result":{"complete":false,"brand":"Acme","channel":"SMS_EMS",` +
	`"notice":{"code":"slow","message":"be patient"},` +
	`"control":{"text":"Orig","performance":50},` +
	`"alternatives":[{"rank":2,"text":"B","performance":60},{"rank":1,"text":"A","performance":70}]}`

func TestJobCommand(t *testing.T) {
	fastPoll(t)
	tests := []struct {
		name       string
		args       []string
		responses  []string
		wantCode   int
		wantOut    []string
		notOut     []string
		wantErr    []string
		wantPaths  int
		wantRawOut string
	}{
		{
			name:      "completed job text",
			args:      []string{testJobID},
			responses: []string{ctJob("completed", ctCompletedResult)},
			wantOut:   []string{testJobID, "completed", "Acme", "Variants", "80", "HIGH", "4/5", "LOW", "Hello / world", "pending"},
			wantErr:   []string{"Persado Martech API"},
			wantPaths: 1,
		},
		{
			name:      "partial optimize result with control and ranked alternatives",
			args:      []string{testJobID, "--no-wait"},
			responses: []string{ctJob("running", ctOptimizedResult)},
			wantOut:   []string{"partial: the job is still running", "Notice", "slow: be patient", "Original", "Alternatives (best first)"},
			wantPaths: 1,
		},
		{
			name:      "no-wait does not poll a running job",
			args:      []string{testJobID, "--no-wait"},
			responses: []string{ctJob("running", ""), ctJob("completed", "")},
			wantOut:   []string{"running"},
			notOut:    []string{"completed"},
			wantPaths: 1,
		},
		{
			name:      "waits with polls until completed",
			args:      []string{testJobID},
			responses: []string{ctJob("queued", ""), ctJob("running", ""), ctJob("completed", ctCompletedResult)},
			wantOut:   []string{"completed", "Variants"},
			wantErr:   []string{"Waiting for score " + testJobID + ".."},
			wantPaths: 3,
		},
		{
			name:      "timeout tells how to read it later",
			args:      []string{testJobID, "--timeout", "30ms"},
			responses: []string{ctJob("running", "")},
			wantCode:  1,
			wantErr:   []string{"still running after 30ms", "persadomapicli job " + testJobID},
			wantPaths: -1,
		},
		{
			name:      "failed job exits 1 and shows the error",
			args:      []string{testJobID},
			responses: []string{ctJob("failed", `"error":{"code":"pii","message":"personal data","categories":["email","phone"]}`)},
			wantCode:  1,
			wantOut:   []string{"failed", "Error", "pii: personal data", "Personal data", "email, phone"},
			wantErr:   []string{"error: job failed"},
			wantPaths: 1,
		},
		{
			name:      "failed job after waiting",
			args:      []string{testJobID},
			responses: []string{ctJob("running", ""), ctJob("failed", `"error":{"code":"boom","message":"it broke"}`)},
			wantCode:  1,
			wantOut:   []string{"boom: it broke"},
			notOut:    []string{"Personal data"},
			wantErr:   []string{"error: job failed"},
			wantPaths: 2,
		},
		{
			name:       "json output is the raw server JSON",
			args:       []string{testJobID, "-o", "json"},
			responses:  []string{ctJob("completed", `"extra":{"keep":true}`)},
			wantOut:    []string{`"status": "completed"`, `"keep": true`},
			notOut:     []string{"Kind"},
			wantPaths:  1,
			wantRawOut: "{\n  \"id\"",
		},
		{
			name:      "json output of a failed job still exits 1",
			args:      []string{testJobID, "-o", "json"},
			responses: []string{ctJob("failed", "")},
			wantCode:  1,
			wantOut:   []string{`"status": "failed"`},
			wantPaths: 1,
		},
		{
			name:      "api error is shown",
			args:      []string{testJobID},
			responses: []string{`HTTP 404 {"error":{"code":"not_found","message":"no such job","requestId":"r1"}}`},
			wantCode:  1,
			wantErr:   []string{"job " + testJobID + " not found: jobs are kept for 24 hours", "no such job (HTTP 404, not_found)", "request id: r1"},
			wantPaths: 1,
		},
		{
			name:      "invalid id is rejected before any request",
			args:      []string{"job_xyz"},
			responses: []string{ctJob("completed", "")},
			wantCode:  1,
			wantErr:   []string{"invalid request", `"job_xyz" is not a job id`},
			wantPaths: 0,
		},
		{
			name:      "missing id is rejected",
			args:      nil,
			responses: []string{ctJob("completed", "")},
			wantCode:  1,
			wantErr:   []string{"is not a job id"},
			wantPaths: 0,
		},
		{
			name:      "too many args",
			args:      []string{testJobID, "extra"},
			responses: []string{ctJob("completed", "")},
			wantCode:  1,
			wantErr:   []string{"accepts at most 1 arg"},
			wantPaths: 0,
		},
		{
			name:      "bad output format fails before any request",
			args:      []string{testJobID, "-o", "yaml"},
			responses: []string{ctJob("completed", "")},
			wantCode:  1,
			wantErr:   []string{"--output must be text or json"},
			wantPaths: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := ctJobServer(t, tc.responses...)
			if strings.HasPrefix(tc.name, "timeout") {
				// The deadline must fire while waiting between polls (see the report on mid-request expiry).
				pollInterval = 200 * time.Millisecond
			}
			args := append([]string{"job"}, tc.args...)
			args = append(args, "--base-url", srv.URL, "--api-key", "pmk_test")
			code, stdout, stderr := runCLI(args...)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", code, tc.wantCode, stdout, stderr)
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(stdout, w) {
					t.Errorf("stdout lacks %q:\n%s", w, stdout)
				}
			}
			for _, w := range tc.notOut {
				if strings.Contains(stdout, w) {
					t.Errorf("stdout must not contain %q:\n%s", w, stdout)
				}
			}
			for _, w := range tc.wantErr {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr lacks %q:\n%s", w, stderr)
				}
			}
			if tc.wantRawOut != "" && !strings.HasPrefix(stdout, tc.wantRawOut) {
				t.Errorf("stdout = %q, want prefix %q", stdout, tc.wantRawOut)
			}
			if tc.wantPaths >= 0 && len(*seen) != tc.wantPaths {
				t.Errorf("server saw %d requests, want %d", len(*seen), tc.wantPaths)
			}
			if len(*seen) > 0 {
				r := (*seen)[0]
				if r.Method != http.MethodGet || r.Path != "/v1/jobs/"+testJobID || r.Auth != "Bearer pmk_test" {
					t.Errorf("first request = %+v", r)
				}
			}
		})
	}
}

func TestJobCommandEdit(t *testing.T) {
	fastPoll(t)
	tests := []struct {
		name     string
		editor   func(ctx context.Context, title, text string, v editor.Validator) (string, error)
		wantCode int
		wantErr  string
		wantOut  string
		wantReqs int
	}{
		{
			name: "edit success",
			editor: func(_ context.Context, title, text string, v editor.Validator) (string, error) {
				if title != "Read a job" || !strings.Contains(text, "// Read a job") || !strings.Contains(text, `"id": ""`) {
					t.Errorf("unexpected editor input %q %q", title, text)
				}
				out := `{"id":"` + testJobID + `"}`
				return out, v(out)
			},
			wantOut:  "completed",
			wantReqs: 1,
		},
		{
			name: "edit validation error",
			editor: func(_ context.Context, _ string, _ string, v editor.Validator) (string, error) {
				return `{"id":"bad"}`, v(`{"id":"bad"}`)
			},
			wantCode: 1,
			wantErr:  "not a job id",
		},
		{
			name: "edit canceled",
			editor: func(context.Context, string, string, editor.Validator) (string, error) {
				return "", editor.ErrCanceled
			},
			wantCode: 1,
			wantErr:  "edit canceled",
		},
		{
			name: "edit empty text",
			editor: func(_ context.Context, _ string, _ string, v editor.Validator) (string, error) {
				return "", v("")
			},
			wantCode: 1,
			wantErr:  "the request is empty",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubEditor(t, tc.editor)
			srv, seen := ctJobServer(t, ctJob("completed", ""))
			code, stdout, stderr := runCLI("job", "-e", "--base-url", srv.URL, "--api-key", "pmk_test")
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %s)", code, tc.wantCode, stderr)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Errorf("stderr %q lacks %q", stderr, tc.wantErr)
			}
			if !strings.Contains(stdout, tc.wantOut) {
				t.Errorf("stdout %q lacks %q", stdout, tc.wantOut)
			}
			if len(*seen) != tc.wantReqs {
				t.Errorf("server saw %d requests, want %d", len(*seen), tc.wantReqs)
			}
		})
	}
}

func TestJobRequestValidate(t *testing.T) {
	tests := []struct {
		id      string
		wantErr bool
	}{
		{testJobID, false},
		{"", true},
		{"job_ABC", true},
		{"job_0123456789abcdef0123456789abcdeg", true},
		{testJobID + "0", true},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			err := (&jobRequest{ID: tc.id}).Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			var ve api.ValidationError
			if err != nil && (!errors.As(err, &ve) || ve[0].Field != "id") {
				t.Errorf("want ValidationError on id, got %v", err)
			}
		})
	}
}

func TestJobTemplate(t *testing.T) {
	got := jobTemplate(&jobRequest{ID: "abc"})
	for _, want := range []string{"// Read a job.", `"id": "abc"`} {
		if !strings.Contains(got, want) {
			t.Errorf("template lacks %q:\n%s", want, got)
		}
	}
}

func TestJobFlags(t *testing.T) {
	cmd := newJobCmd(&globalFlags{})
	if cmd.Flags().Lookup("no-wait") == nil || cmd.Flags().Lookup("timeout") == nil {
		t.Fatal("job needs --no-wait and --timeout")
	}
	if cmd.Flags().Lookup("async") != nil {
		t.Error("job must not have --async")
	}
	if f := cmd.Flags().Lookup("timeout"); f.DefValue != (15 * time.Minute).String() {
		t.Errorf("timeout default = %s", f.DefValue)
	}
	if f := newScoreCmd(&globalFlags{}).Flags().Lookup("async"); f == nil {
		t.Error("score needs --async")
	}
}

func TestWaitJob(t *testing.T) {
	fastPoll(t)
	t.Run("returns raw of the last read", func(t *testing.T) {
		srv, _ := ctJobServer(t, ctJob("running", ""), ctJob("completed", ""))
		c := &api.Client{BaseURL: srv.URL, Key: "k"}
		var progress bytes.Buffer
		job, raw, err := waitJob(context.Background(), c, &api.Job{ID: testJobID, Kind: "score", Status: "queued"}, time.Minute, &progress)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status != "completed" || !strings.Contains(string(raw), `"completed"`) {
			t.Errorf("job %+v raw %s", job, raw)
		}
		if got := progress.String(); got != "Waiting for score "+testJobID+"..\n" {
			t.Errorf("progress = %q", got)
		}
	})
	t.Run("parent context canceled", func(t *testing.T) {
		srv, _ := ctJobServer(t, ctJob("running", ""))
		c := &api.Client{BaseURL: srv.URL, Key: "k"}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, err := waitJob(ctx, c, &api.Job{ID: testJobID, Status: "running"}, time.Minute, &bytes.Buffer{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("api error while polling", func(t *testing.T) {
		srv, _ := ctJobServer(t, `HTTP 500 {"error":{"message":"down"}}`)
		c := &api.Client{BaseURL: srv.URL, Key: "k"}
		_, _, err := waitJob(context.Background(), c, &api.Job{ID: testJobID, Status: "running"}, time.Minute, &bytes.Buffer{})
		var ae *api.Error
		if !errors.As(err, &ae) || ae.Status != 500 {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRunJob(t *testing.T) {
	fastPoll(t)
	tests := []struct {
		name      string
		async     bool
		startErr  error
		responses []string
		wantWait  int
		wantErr   string
		wantOut   string
	}{
		{"waits for the job", false, nil, []string{ctJob("completed", "")}, api.MaxWaitSeconds, "", "completed"},
		{"async skips polling and waiting", true, nil, []string{ctJob("running", "")}, 0, "", "Next     persadomapicli job " + testJobID},
		{"start error", false, errors.New("boom"), nil, api.MaxWaitSeconds, "boom", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var srv *httptest.Server
			var seen *[]seenRequest
			if tc.responses != nil {
				srv, seen = ctJobServer(t, tc.responses...)
			}
			var out, errb bytes.Buffer
			cmd := newJobCmd(&globalFlags{output: outputText})
			cmd.SetOut(&out)
			cmd.SetErr(&errb)
			c := &api.Client{Key: "k"}
			if srv != nil {
				c.BaseURL = srv.URL
			}
			var gotWait int
			err := runJob(cmd, &globalFlags{output: outputText}, waitFlags{async: tc.async, timeout: time.Minute}, c,
				func(wait int) (*api.Response[api.Job], error) {
					gotWait = wait
					if tc.startErr != nil {
						return nil, tc.startErr
					}
					return c.Job(context.Background(), testJobID)
				})
			if gotWait != tc.wantWait {
				t.Errorf("wait = %d, want %d", gotWait, tc.wantWait)
			}
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.wantOut) {
				t.Errorf("stdout lacks %q:\n%s", tc.wantOut, out.String())
			}
			if tc.async && len(*seen) != 1 {
				t.Errorf("async made %d requests, want 1", len(*seen))
			}
		})
	}
}

func TestPrintJob(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		job     *api.Job
		raw     string
		wantOut string
		wantErr error
	}{
		{"text completed", outputText, &api.Job{ID: "j", Kind: "score", Status: api.JobCompleted}, `{}`, "Status   completed", nil},
		{"text failed", outputText, &api.Job{ID: "j", Status: api.JobFailed}, `{}`, "failed", errJobFailed},
		{"json completed", outputJSON, &api.Job{Status: api.JobCompleted}, `{"a":1}`, "{\n  \"a\": 1\n}\n", nil},
		{"json failed", outputJSON, &api.Job{Status: api.JobFailed}, `{"a":1}`, `"a": 1`, errJobFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			err := printJob(&b, tc.format, tc.job, []byte(tc.raw))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if !strings.Contains(b.String(), tc.wantOut) {
				t.Errorf("out %q lacks %q", b.String(), tc.wantOut)
			}
		})
	}
}

type failWriter struct{ after int }

func (f *failWriter) Write(p []byte) (int, error) {
	if f.after <= 0 {
		return 0, errors.New("write failed")
	}
	f.after--
	return len(p), nil
}

func TestPrintJobWriteError(t *testing.T) {
	for _, format := range []string{outputText, outputJSON} {
		err := printJob(&failWriter{}, format, &api.Job{Status: api.JobCompleted}, []byte(`{}`))
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Errorf("%s: err = %v", format, err)
		}
	}
	// Failures later in renderJob (after the header flushed).
	job := &api.Job{Result: &api.Result{Complete: true, Variants: []api.Variant{{Text: "x"}}}}
	if err := renderJob(&failWriter{after: 1}, job); err == nil {
		t.Error("expected a write error from the variants table")
	}
	job = &api.Job{Result: &api.Result{Complete: true, Control: &api.Variant{Text: "x"}}}
	if err := renderJob(&failWriter{after: 1}, job); err == nil {
		t.Error("expected a write error from the control table")
	}
}

func TestRenderJob(t *testing.T) {
	one, two := 1, 2
	band := "HIGH"
	tests := []struct {
		name string
		job  *api.Job
		want []string
		not  []string
	}{
		{
			name: "no result",
			job:  &api.Job{ID: "j", Kind: "optimize", Status: "queued", CreatedAt: "c", ExpiresAt: "e"},
			want: []string{"Job      j", "Kind     optimize", "Created  c", "Expires  e", "Next     persadomapicli job j"},
			not:  []string{"Brand", "Variants"},
		},
		{
			name: "complete result without variants",
			job:  &api.Job{Status: api.JobCompleted, Result: &api.Result{Complete: true, Brand: "B", Channel: "C"}},
			want: []string{"Brand       B", "Channel     C", "Compliance  not applicable to this request"},
			not:  []string{"partial", "Variants", "Alternatives", "Next"},
		},
		{
			name: "alternatives sorted by rank, nil rank first, no control",
			job: &api.Job{Result: &api.Result{Complete: true, Alternatives: []api.Variant{
				{Rank: &two, Text: "second", PerformanceBand: &band}, {Rank: &one, Text: "first"}, {Text: "norank"}}}},
			want: []string{"Alternatives (best first)", "Rank"},
			not:  []string{"Original"},
		},
		{
			name: "compliance risk preferred over state",
			job:  &api.Job{Result: &api.Result{Variants: []api.Variant{{Index: &one, Text: "t", Compliance: &api.Compliance{Risk: "HIGH", State: "done"}}}}},
			want: []string{"HIGH"},
			not:  []string{"done"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := renderJob(&b, tc.job); err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(b.String(), w) {
					t.Errorf("output lacks %q:\n%s", w, b.String())
				}
			}
			for _, w := range tc.not {
				if strings.Contains(b.String(), w) {
					t.Errorf("output must not contain %q:\n%s", w, b.String())
				}
			}
		})
	}
	t.Run("alternative order", func(t *testing.T) {
		job := &api.Job{Result: &api.Result{Alternatives: []api.Variant{{Rank: &two, Text: "BBB"}, {Rank: &one, Text: "AAA"}}}}
		var b bytes.Buffer
		if err := renderJob(&b, job); err != nil {
			t.Fatal(err)
		}
		if strings.Index(b.String(), "AAA") > strings.Index(b.String(), "BBB") {
			t.Errorf("rank 1 must come first:\n%s", b.String())
		}
	})
}

func TestDeref(t *testing.T) {
	n := 7
	if deref(nil) != 0 || deref(&n) != 7 {
		t.Error("deref")
	}
}
