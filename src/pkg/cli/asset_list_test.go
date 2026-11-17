package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/editor"
)

func TestAssetList(t *testing.T) {
	pages := map[string]string{
		"":    `{"assets":[{"id":"a1","filename":"one.txt","mimeType":"text/plain","size":3,"version":1,"createdAt":"c1","expiresAt":"e1","attributes":{"b":"2","a":"1"}}],"nextCursor":"c-2"}`,
		"c-2": `{"assets":[],"nextCursor":"c-3"}`,
		"c-3": `{"assets":[{"id":"a2","filename":"two.txt","mimeType":"text/plain","size":4,"version":1,"createdAt":"c2","expiresAt":"e2","attributes":{}}]}`,
	}
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/asset/list" || r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer pmk_test" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		queries = append(queries, r.URL.RawQuery)
		body, ok := pages[r.URL.Query().Get("cursor")]
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"bad"}}`))
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	base := []string{"--base-url", srv.URL, "--api-key", "pmk_test", "asset", "list"}

	tests := []struct {
		name        string
		args        []string
		wantCode    int
		wantOut     []string
		notOut      []string
		wantErr     string
		wantQueries []string
	}{
		{"first page", nil, 0, []string{"ID", "Attributes", "one.txt", "a=1,b=2"}, []string{"two.txt"}, "--cursor c-2", []string{""}},
		{"limit and cursor", []string{"--limit", "5", "--cursor", "c-3"}, 0, []string{"two.txt", " -"}, nil, "", []string{"cursor=c-3&limit=5"}},
		{"all follows empty pages", []string{"--all"}, 0, []string{"one.txt", "two.txt"}, nil, "", []string{"", "cursor=c-2", "cursor=c-3"}},
		{"all json", []string{"--all", "-o", "json"}, 0, []string{`"assets"`, `"one.txt"`, `"two.txt"`}, []string{"nextCursor"}, "", []string{"", "cursor=c-2", "cursor=c-3"}},
		{"page json raw", []string{"-o", "json"}, 0, []string{`"nextCursor": "c-2"`}, nil, "--cursor c-2", []string{""}},
		{"limit too high", []string{"--limit", "201"}, 1, nil, nil, "limit: must be 1 to 200", nil},
		{"limit negative", []string{"--limit", "-1"}, 1, nil, nil, "limit", nil},
		{"all with cursor", []string{"--all", "--cursor", "x"}, 1, nil, nil, "cannot be combined", nil},
		{"cursor too long", []string{"--cursor", strings.Repeat("x", 513)}, 1, nil, nil, "at most 512", nil},
		{"server error", []string{"--cursor", "nope"}, 1, nil, nil, "bad (HTTP 400", []string{"cursor=nope"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			queries = nil
			code, out, errOut := runCLI(append(append([]string{}, base...), tc.args...)...)
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, tc.wantCode, errOut)
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(out, w) {
					t.Errorf("stdout lacks %q:\n%s", w, out)
				}
			}
			for _, w := range tc.notOut {
				if strings.Contains(out, w) {
					t.Errorf("stdout has %q:\n%s", w, out)
				}
			}
			if !strings.Contains(errOut, tc.wantErr) {
				t.Errorf("stderr %q lacks %q", errOut, tc.wantErr)
			}
			if strings.Join(queries, "|") != strings.Join(tc.wantQueries, "|") {
				t.Errorf("queries = %q, want %q", queries, tc.wantQueries)
			}
		})
	}

	t.Run("all json is valid and empty list is an array", func(t *testing.T) {
		pages[""] = `{"assets":[]}`
		defer func() { pages[""] = `{"assets":[],"nextCursor":"c-2"}` }()
		_, out, _ := runCLI(append(append([]string{}, base...), "--all", "-o", "json")...)
		var v struct{ Assets []any }
		if err := json.Unmarshal([]byte(out), &v); err != nil || v.Assets == nil {
			t.Errorf("out %q: err %v assets %v", out, err, v.Assets)
		}
	})

	t.Run("no key", func(t *testing.T) {
		t.Setenv(envAPIKey, "")
		code, _, errOut := runCLI("--base-url", srv.URL, "asset", "list")
		if code != 1 || !strings.Contains(errOut, "no API key") {
			t.Errorf("code %d stderr %q", code, errOut)
		}
	})
}

func TestAssetListEdit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.RawQuery; got != "limit=7" {
			t.Errorf("query = %q", got)
		}
		_, _ = w.Write([]byte(`{"assets":[]}`))
	}))
	defer srv.Close()
	args := []string{"--base-url", srv.URL, "--api-key", "pmk_test", "asset", "list", "--edit", "--limit", "3"}

	t.Run("success", func(t *testing.T) {
		stubEditor(t, func(_ context.Context, _, text string, v editor.Validator) (string, error) {
			if !strings.Contains(text, `"limit": 3`) || !strings.Contains(text, "// ") {
				t.Errorf("template %q", text)
			}
			edited := `{"limit":7,"cursor":"","all":false}`
			return edited, v(edited)
		})
		if code, _, errOut := runCLI(args...); code != 0 {
			t.Fatalf("exit %d: %s", code, errOut)
		}
	})
	t.Run("validation error", func(t *testing.T) {
		stubEditor(t, func(_ context.Context, _, _ string, v editor.Validator) (string, error) {
			bad := `{"limit":999}`
			return bad, v(bad)
		})
		code, _, errOut := runCLI(args...)
		if code != 1 || !strings.Contains(errOut, "limit") {
			t.Errorf("code %d stderr %q", code, errOut)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		stubEditor(t, func(context.Context, string, string, editor.Validator) (string, error) { return "", editor.ErrCanceled })
		code, _, errOut := runCLI(args...)
		if code != 1 || errOut == "" {
			t.Errorf("code %d stderr %q", code, errOut)
		}
		_ = errors.Is
	})
}

func TestShortTime(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-17T22:45:45.792929663Z": "2026-11-17T22:45:45Z",
		"2026-11-18T22:45:45Z":           "2026-11-18T22:45:45Z",
		"2026-11-18T00:45:45.5+02:00":    "2026-11-17T22:45:45Z",
		"c1":                             "c1",
	} {
		if got := shortTime(in); got != want {
			t.Errorf("shortTime(%q) = %q, want %q", in, got, want)
		}
	}
}
