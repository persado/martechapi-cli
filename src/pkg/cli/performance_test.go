package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/api"
)

const perfBody = `{"model":"m1","resolvedLanguage":"en","localeSource":"account","clientModel":false,` +
	`"results":[{"text":"Hello","performance":12.5,"model":"m1"},{"text":"Bye\nnow","performance":null,"model":"m1"}]}`

func TestPerformanceCommand(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.URL.Path != "/v1/performance" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(perfBody))
	}))
	defer srv.Close()
	base := []string{"--api-key", "k", "--base-url", srv.URL, "performance", "-a", "Acme", "-v", "Hello", "-v", `{"text":"Bye"}`}

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  []string
		wantErr  string
	}{
		{"text", base, 0, []string{"Model          m1", "Language       en", "Locale source  account", "Performance (%)", "12.50", "Bye / now", "-"}, ""},
		{"json", append([]string{"-o", "json"}, base...), 0, []string{`"performance": 12.5`, `"performance": null`}, ""},
		{"no variants", []string{"--api-key", "k", "--base-url", srv.URL, "performance", "-a", "Acme"}, 1, nil, "variants"},
		{"bad variant", []string{"--api-key", "k", "--base-url", srv.URL, "performance", "-v", "{bad"}, 1, nil, "error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := Run(context.Background(), tc.args, &out, &errb)
			if code != tc.wantCode {
				t.Fatalf("code = %d, want %d (stderr %q)", code, tc.wantCode, errb.String())
			}
			for _, s := range tc.wantOut {
				if !strings.Contains(out.String(), s) {
					t.Errorf("stdout %q lacks %q", out.String(), s)
				}
			}
			if !strings.Contains(errb.String(), tc.wantErr) {
				t.Errorf("stderr %q lacks %q", errb.String(), tc.wantErr)
			}
		})
	}
	if !strings.Contains(gotBody, `"variants"`) {
		t.Errorf("request body %q lacks variants", gotBody)
	}
}

func TestPerformanceTemplate(t *testing.T) {
	empty := performanceTemplate(&api.PerformanceRequest{})
	if !strings.Contains(empty, `"text"`) || !strings.Contains(empty, "1 to 10") {
		t.Errorf("empty template: %s", empty)
	}
	full := performanceTemplate(&api.PerformanceRequest{Variants: []api.Touchpoints{{"text": "Hi there"}}})
	if !strings.Contains(full, "Hi there") {
		t.Errorf("template lacks variant: %s", full)
	}
}

func TestRenderPerformance(t *testing.T) {
	v := 3.14159
	var out bytes.Buffer
	err := renderPerformance(&out, &api.PerformanceResponse{Model: "m", Results: []api.PerformanceResult{{Text: "a", Performance: &v, Model: "m"}, {Text: "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"3.14", "0  ", "1  -"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("output %q lacks %q", out.String(), s)
		}
	}
	out.Reset()
	if err := renderPerformance(&out, &api.PerformanceResponse{Results: []api.PerformanceResult{{Text: "b"}}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Model          -") {
		t.Errorf("empty model not dashed: %q", out.String())
	}
}
