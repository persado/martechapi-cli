package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/editor"
)

const optJobID = "job_0123456789abcdef0123456789abcdef"

func optJob(status string) string {
	return `{"id":"` + optJobID + `","kind":"optimize","status":"` + status + `","createdAt":"c","updatedAt":"u","expiresAt":"e",` +
		`"result":{"brand":"Acme","channel":"EMAIL_SUBJECT","complete":true,"control":{"text":"orig","performance":50},` +
		`"alternatives":[{"text":"alt one","rank":1,"performance":80}]},"error":null}`
}

type optSrv struct {
	body  map[string]any
	query string
	polls int
}

func newOptSrv(t *testing.T, post func(w http.ResponseWriter), poll string) (*httptest.Server, *optSrv) {
	t.Helper()
	s := &optSrv{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/v1/optimize" {
			s.query = r.URL.RawQuery
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &s.body)
			post(w)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/v1/jobs/"+optJobID {
			s.polls++
			_, _ = io.WriteString(w, poll)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, s
}

func TestOptimizeFlags(t *testing.T) {
	brandPath := filepath.Join(t.TempDir(), "brand.json")
	if err := os.WriteFile(brandPath, []byte(`{"styleSetName":"file","exclusionList":["a"],"characterLimits":{"text":50}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, s := newOptSrv(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, optJob("completed"))
	}, "")
	code, out, errb := runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "Acme", "-v", "Hello there", "--context", "spring", "--always-use", "0% APR", "--always-use", "free",
		"--char-limit", "TEXT=40,Other=9", "--brand-file", brandPath, "--style-set", "mine", "--exclude", "bad", "--exclude", "worse",
		"--brand-char-limit", "text=60", "--assistant-id", "as1", "--content-seeding")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	for _, want := range []string{"alt one", "orig", "Alternatives"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	b, _ := json.Marshal(s.body)
	want := `{"account":{"name":"Acme"},"alwaysUse":["0% APR","free"],"brand":{"characterLimits":{"text":60},"exclusionList":["bad","worse"],"styleSetName":"mine"},` +
		`"characterLimits":{"other":9,"text":40},"context":"spring","generation":{"assistantId":"as1","contentSeedingEnabled":true},"variant":{"text":"Hello there"}}`
	if string(b) != want {
		t.Errorf("body\n got %s\nwant %s", b, want)
	}
	if s.query != "wait=50" {
		t.Errorf("query = %q", s.query)
	}
}

func TestOptimizeBrandFileOnly(t *testing.T) {
	brandPath := filepath.Join(t.TempDir(), "brand.json")
	_ = os.WriteFile(brandPath, []byte(`{"styleGuidelines":{"Tone of Voice":[{"style":"Warm"}]}}`), 0o600)
	srv, s := newOptSrv(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, optJob("completed")) }, "")
	code, _, errb := runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "Acme", "-v", "Hi", "--brand-file", brandPath, "--content-seeding=false")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	brand := s.body["brand"].(map[string]any)
	if _, ok := brand["styleGuidelines"]; !ok {
		t.Errorf("brand = %v", brand)
	}
	if g := s.body["generation"].(map[string]any); g["contentSeedingEnabled"] != false {
		t.Errorf("generation = %v", g)
	}
}

func TestOptimizeFileAndStructuredVariant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "req.json")
	_ = os.WriteFile(path, []byte(`{"channel":"PUSH_NOTIFICATION","account":{"name":"Acme"},"variant":{"headline":"x","body":"y"},"generation":{"assistantId":"keep","contentSeedingEnabled":true}}`), 0o600)
	srv, s := newOptSrv(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, optJob("completed")) }, "")
	code, _, errb := runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "--file", path, "-v", `{"headline":"h","body":"b"}`, "--content-seeding=false")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	g := s.body["generation"].(map[string]any)
	if g["assistantId"] != "keep" || g["contentSeedingEnabled"] != false {
		t.Errorf("generation = %v", g)
	}
	if v := s.body["variant"].(map[string]any); v["headline"] != "h" {
		t.Errorf("variant = %v", v)
	}
}

func TestOptimizeAsyncAndPolling(t *testing.T) {
	fastPoll(t)
	for _, tc := range []struct {
		name  string
		args  []string
		polls int
		query string
		want  string
	}{
		{"async prints job only", []string{"--async"}, 0, "", optJobID},
		{"polls until done", nil, 1, "wait=50", "alt one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, s := newOptSrv(t, func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = io.WriteString(w, strings.Replace(optJob("running"), `"complete":true`, `"complete":false`, 1))
			}, optJob("completed"))
			code, out, errb := runCLI(append([]string{"optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "Acme", "-v", "Hi"}, tc.args...)...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errb)
			}
			if s.polls != tc.polls || s.query != tc.query || !strings.Contains(out, tc.want) {
				t.Errorf("polls=%d query=%q out=%s", s.polls, s.query, out)
			}
		})
	}
}

func TestOptimizeJSONAndFailedJob(t *testing.T) {
	srv, _ := newOptSrv(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, optJob("completed")) }, "")
	code, out, _ := runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "Acme", "-v", "Hi", "-o", "json")
	if code != 0 || !strings.Contains(out, `"kind": "optimize"`) {
		t.Errorf("json: exit %d out %s", code, out)
	}
	failed := `{"id":"` + optJobID + `","kind":"optimize","status":"failed","createdAt":"c","updatedAt":"u","expiresAt":"e","result":null,"error":{"code":"internal","message":"boom"}}`
	srv, _ = newOptSrv(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, failed) }, "")
	code, out, _ = runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "Acme", "-v", "Hi")
	if code != 1 || !strings.Contains(out, "boom") {
		t.Errorf("failed: exit %d out %s", code, out)
	}
}

func TestOptimizeErrors(t *testing.T) {
	srv, _ := newOptSrv(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":"invalid_request","message":"bad","requestId":"r"}}`)
	}, "")
	dir := t.TempDir()
	badBrand := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(badBrand, []byte(`{"nope":1}`), 0o600)
	badFile := filepath.Join(dir, "badreq.json")
	_ = os.WriteFile(badFile, []byte(`{"nope":1}`), 0o600)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing variant", []string{"-a", "Acme"}, "variant"},
		{"too many always-use", []string{"-a", "A", "-v", "x", "--always-use", "1", "--always-use", "2", "--always-use", "3", "--always-use", "4", "--always-use", "5", "--always-use", "6", "--always-use", "7", "--always-use", "8", "--always-use", "9", "--always-use", "10", "--always-use", "11"}, "alwaysUse"},
		{"bad limit", []string{"-a", "A", "-v", "x", "--char-limit", "text=0"}, "characterLimits.text"},
		{"bad variant json", []string{"-a", "A", "-v", `{"x":`}, "--variant"},
		{"brand file missing", []string{"-a", "A", "-v", "x", "--brand-file", filepath.Join(dir, "none")}, "read request"},
		{"brand file unknown field", []string{"-a", "A", "-v", "x", "--brand-file", badBrand}, "invalid JSON"},
		{"request file unknown field", []string{"--file", badFile}, "invalid JSON"},
		{"server rejects", []string{"-a", "A", "-v", "x"}, "bad"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errb := runCLI(append([]string{"optimize", "--base-url", srv.URL, "--api-key", "pmk_test"}, tc.args...)...)
			if code != 1 || !strings.Contains(errb, tc.want) {
				t.Errorf("exit %d stderr %s", code, errb)
			}
		})
	}
}

func TestOptimizeEdit(t *testing.T) {
	srv, s := newOptSrv(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, optJob("completed")) }, "")
	t.Run("success", func(t *testing.T) {
		stubEditor(t, func(_ context.Context, _, text string, v editor.Validator) (string, error) {
			for _, part := range []string{`"generation": null`, `"styleGuidelines": {}`, `"alwaysUse": []`, `"context": ""`, `"text": ""`, "// generation is null unless set", "// Touchpoint keys"} {
				if !strings.Contains(text, part) {
					t.Errorf("template missing %q:\n%s", part, text)
				}
			}
			edited := `{"account":{"name":"Edited"},"variant":{"text":"Hi"}}`
			return edited, v(edited)
		})
		code, _, errb := runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "--edit")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, errb)
		}
		if s.body["account"].(map[string]any)["name"] != "Edited" {
			t.Errorf("body = %v", s.body)
		}
	})
	t.Run("template carries flags and brand", func(t *testing.T) {
		stubEditor(t, func(_ context.Context, _, text string, v editor.Validator) (string, error) {
			for _, part := range []string{`"styleSetName": "mine"`, `"bad"`, `"text": 40`, `"generation": {`, `"contentSeedingEnabled": true`} {
				if !strings.Contains(text, part) {
					t.Errorf("template missing %q:\n%s", part, text)
				}
			}
			return `{"account":{"name":"A"},"variant":{"text":"Hi"}}`, nil
		})
		code, _, errb := runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "A", "-v", "Hi", "--style-set", "mine", "--exclude", "bad", "--char-limit", "text=40", "--content-seeding", "--edit")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, errb)
		}
	})
	t.Run("validation error", func(t *testing.T) {
		stubEditor(t, func(_ context.Context, _, _ string, v editor.Validator) (string, error) {
			return "", v(`{"account":{"name":""},"variant":{"text":"Hi"}}`)
		})
		code, _, errb := runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "--edit")
		if code != 1 || !strings.Contains(errb, "account.name") {
			t.Errorf("exit %d stderr %s", code, errb)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		stubEditor(t, func(context.Context, string, string, editor.Validator) (string, error) { return "", editor.ErrCanceled })
		code, _, errb := runCLI("optimize", "--base-url", srv.URL, "--api-key", "pmk_test", "--edit")
		if code != 1 || errb == "" {
			t.Errorf("exit %d stderr %q", code, errb)
		}
	})
}

func TestOptimizeHelpers(t *testing.T) {
	if got := mergeLimits(nil, map[string]int{" A ": 1}); got["a"] != 1 {
		t.Errorf("mergeLimits = %v", got)
	}
	if got := orEmpty(nil); got == nil || len(got) != 0 {
		t.Errorf("orEmpty = %v", got)
	}
}
