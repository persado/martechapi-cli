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

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/persado/martechapi-cli/src/pkg/editor"
)

const upliftBody = `{"performance":15.69,"potential":71.39,"atPotential":false,"resolution":"triplet","channelCategory":"email","vertical":"fin",
"notice":{"code":"not_scored","message":"no score"},
"uplift":[{"category":"Click","vsIndustryToday":-3.2,"vsIndustryProjected":12.5,"total":16.4,"unavailable":null},
{"category":"Open","vsIndustryToday":null,"vsIndustryProjected":null,"total":null,"unavailable":"no data"}]}`

func upliftServer(t *testing.T, status int, body string, got *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/uplift" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer pmk_test" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		if got != nil {
			*got = map[string]any{}
			_ = json.Unmarshal(b, got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestUpliftText(t *testing.T) {
	var got map[string]any
	srv := upliftServer(t, 200, upliftBody, &got)
	code, out, _ := runCLI("uplift", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "Acme", "-v", "Hello", "--category", "click", "--category", "OPEN")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"15.69", "71.39", "triplet", "email", "not_scored: no score", "-3.2%", "+12.5%", "+16.4%", "- (no data)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	cats, _ := got["responseCategories"].([]any)
	if len(cats) != 2 || cats[0] != "Click" || cats[1] != "Open" {
		t.Errorf("categories = %v", cats)
	}
	if v, _ := got["variant"].(map[string]any); v["text"] != "Hello" {
		t.Errorf("variant = %v", got["variant"])
	}
}

func TestUpliftJSONAndEmptyResult(t *testing.T) {
	srv := upliftServer(t, 200, `{"performance":null,"potential":null,"atPotential":true,"resolution":null,"channelCategory":null,"vertical":null,"uplift":[]}`, nil)
	code, out, _ := runCLI("uplift", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "Acme", "-v", "Hi", "-o", "json")
	if code != 0 || !strings.Contains(out, `"atPotential": true`) {
		t.Fatalf("json: exit %d %q", code, out)
	}
	code, out, _ = runCLI("uplift", "--base-url", srv.URL, "--api-key", "pmk_test", "-a", "Acme", "-v", "Hi")
	if code != 0 || !strings.Contains(out, "At potential") || strings.Contains(out, "Category") {
		t.Fatalf("text: exit %d %q", code, out)
	}
}

func TestUpliftErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "r.json")
	_ = os.WriteFile(file, []byte(`{"bogus":1}`), 0o600)
	srv := upliftServer(t, 400, `{"code":"invalid_request","message":"bad"}`, nil)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing account", []string{"-v", "Hi"}, "account.name"},
		{"bad category", []string{"-a", "A", "-v", "Hi", "--category", "Zap"}, "responseCategories"},
		{"bad variant json", []string{"-a", "A", "-v", "{x"}, "--variant"},
		{"bad file", []string{"-f", file}, "invalid JSON"},
		{"missing file", []string{"-f", "/nonexistent/x.json"}, "read request"},
		{"server error", []string{"-a", "A", "-v", "Hi"}, "bad"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := runCLI(append([]string{"uplift", "--base-url", srv.URL, "--api-key", "pmk_test"}, tc.args...)...)
			if code != 1 || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d, stderr %q, want %q", code, stderr, tc.want)
			}
		})
	}
}

func TestUpliftFile(t *testing.T) {
	var got map[string]any
	srv := upliftServer(t, 200, upliftBody, &got)
	file := filepath.Join(t.TempDir(), "r.json")
	_ = os.WriteFile(file, []byte(`{"account":{"name":"Acme"},"variant":{"text":"From file"},"responseCategories":["Open"]}`), 0o600)
	if code, _, e := runCLI("uplift", "--base-url", srv.URL, "--api-key", "pmk_test", "-f", file, "--country", "gr"); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	if v, _ := got["variant"].(map[string]any); v["text"] != "From file" {
		t.Errorf("variant = %v", got["variant"])
	}
	if a, _ := got["account"].(map[string]any); a["country"] != "GR" {
		t.Errorf("account = %v", got["account"])
	}
}

func TestUpliftEdit(t *testing.T) {
	var got map[string]any
	srv := upliftServer(t, 200, upliftBody, &got)
	orig := runEditor
	t.Cleanup(func() { runEditor = orig })

	t.Run("saved", func(t *testing.T) {
		runEditor = func(_ context.Context, title, text string, v editor.Validator) (string, error) {
			for _, s := range []string{"// responseCategories: any of Click, Conversion, Open", `"responseCategories": []`, `"name": "Acme"`} {
				if !strings.Contains(text, s) {
					t.Errorf("template missing %q:\n%s", s, text)
				}
			}
			if err := v(""); err == nil {
				t.Error("empty request accepted")
			}
			if err := v(`{"account":{"name":""},"variant":{"text":"x"}}`); err == nil {
				t.Error("invalid request accepted")
			}
			edited := `{"account":{"name":"Acme"},"variant":{"text":"Edited"}}`
			return edited, v(edited)
		}
		code, _, e := runCLI("uplift", "--base-url", srv.URL, "--api-key", "pmk_test", "--edit", "-a", "Acme")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, e)
		}
		if v, _ := got["variant"].(map[string]any); v["text"] != "Edited" {
			t.Errorf("variant = %v", got["variant"])
		}
	})
	t.Run("canceled", func(t *testing.T) {
		runEditor = func(context.Context, string, string, editor.Validator) (string, error) {
			return "", editor.ErrCanceled
		}
		if code, _, e := runCLI("uplift", "--base-url", srv.URL, "--api-key", "pmk_test", "--edit"); code != 1 || e == "" {
			t.Errorf("exit %d stderr %q", code, e)
		}
	})
	t.Run("template defaults", func(t *testing.T) {
		if s := upliftTemplate(&api.UpliftRequest{}); !strings.Contains(s, `"text": ""`) {
			t.Errorf("template: %s", s)
		}
	})
}

func TestNormalizeCategory(t *testing.T) {
	for in, want := range map[string]string{"click": "Click", "CONVERSION": "Conversion", "oPen": "Open", "zap": "zap"} {
		if got := normalizeCategory(in); got != want {
			t.Errorf("normalizeCategory(%q) = %q, want %q", in, got, want)
		}
	}
}
