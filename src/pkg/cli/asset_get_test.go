package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/editor"
)

const testAssetID = "0b9a3a8e-5d1f-4c53-9a77-3f2d6c1e8b40"

func TestAssetGet(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if r.URL.Path != "/v1/asset/retrieve" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("id") == "11111111-1111-1111-1111-111111111111" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"asset not found"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"` + testAssetID + `","url":"https://files.example/x","urlExpiresAt":"2030-01-02T03:04:05.123456789Z","filename":"a/b.pdf","mimeType":"application/pdf","size":12,"version":1,"expiresAt":"E"}`))
	}))
	defer srv.Close()
	base := []string{"--base-url", srv.URL, "--api-key", "pmk_test", "asset", "get"}

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  []string
		wantErr  string
	}{
		{"text", []string{testAssetID}, 0, []string{"2030-01-02T03:04:05Z", "URL  ", "https://files.example/x", "URL expires", "b.pdf"}, "anyone with this URL can read the file"},
		{"json", []string{testAssetID, "-o", "json"}, 0, []string{`"urlExpiresAt": "2030-01-02T03:04:05.123456789Z"`}, "anyone with this URL"},
		{"bad id", []string{"nope"}, 1, nil, "not an asset id"},
		{"no id", nil, 1, nil, "not an asset id"},
		{"not found", []string{"11111111-1111-1111-1111-111111111111"}, 1, nil, "asset 11111111-1111-1111-1111-111111111111 not found or expired; list your assets with: persadomapicli asset list: asset not found (HTTP 404"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := runCLI(append(append([]string{}, base...), tc.args...)...)
			if code != tc.wantCode {
				t.Fatalf("exit %d, want %d (stderr %q)", code, tc.wantCode, errOut)
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(out, w) {
					t.Errorf("stdout lacks %q:\n%s", w, out)
				}
			}
			if !strings.Contains(errOut, tc.wantErr) {
				t.Errorf("stderr %q lacks %q", errOut, tc.wantErr)
			}
		})
	}
	if gotQuery == "" || strings.Contains(gotQuery, "stream") {
		t.Errorf("get must not stream: %q", gotQuery)
	}

	t.Run("edit", func(t *testing.T) {
		stubEditor(t, func(_ context.Context, _, text string, v editor.Validator) (string, error) {
			if !strings.Contains(text, "// ") || !strings.Contains(text, `"id": ""`) {
				t.Errorf("template %q", text)
			}
			bad := `{"id":"x"}`
			if v(bad) == nil {
				t.Error("bad id accepted")
			}
			good := `{"id":"` + testAssetID + `"}`
			return good, v(good)
		})
		code, out, errOut := runCLI(append(append([]string{}, base...), "-e")...)
		if code != 0 || !strings.Contains(out, "b.pdf") {
			t.Errorf("code %d out %q err %q", code, out, errOut)
		}
	})
	t.Run("edit canceled", func(t *testing.T) {
		stubEditor(t, func(context.Context, string, string, editor.Validator) (string, error) { return "", editor.ErrCanceled })
		if code, _, _ := runCLI(append(append([]string{}, base...), "-e")...); code != 1 {
			t.Errorf("code %d", code)
		}
	})
}

func TestTruncSeconds(t *testing.T) {
	for in, want := range map[string]string{
		"2030-01-02T03:04:05.9Z": "2030-01-02T03:04:05Z",
		"2030-01-02T03:04:05Z":   "2030-01-02T03:04:05Z",
		"U":                      "U",
	} {
		if got := truncSeconds(in); got != want {
			t.Errorf("truncSeconds(%q) = %q, want %q", in, got, want)
		}
	}
}
