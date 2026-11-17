package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/editor"
)

func TestAssetDownload(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if r.URL.Query().Get("id") != testAssetID {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"asset not found"}}`))
			return
		}
		w.Header().Set("X-Asset-Filename", "dir/caf%C3%A9%20brief.txt")
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()
	base := []string{"--base-url", srv.URL, "--api-key", "pmk_test", "asset", "download"}

	read := func(t *testing.T, p string) string {
		t.Helper()
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("default name in cwd, then refuse, then force", func(t *testing.T) {
		t.Chdir(t.TempDir())
		code, _, errOut := runCLI(append(append([]string{}, base...), testAssetID)...)
		if code != 0 || !strings.Contains(errOut, "Saved café brief.txt (5 bytes)") {
			t.Fatalf("code %d stderr %q", code, errOut)
		}
		if gotQuery != "id="+testAssetID+"&stream=true" {
			t.Errorf("query %q", gotQuery)
		}
		if read(t, "café brief.txt") != "hello" {
			t.Error("content")
		}
		if err := os.WriteFile("café brief.txt", []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, _, errOut = runCLI(append(append([]string{}, base...), testAssetID)...)
		if code != 1 || !strings.Contains(errOut, "already exists") || read(t, "café brief.txt") != "old" {
			t.Errorf("code %d stderr %q", code, errOut)
		}
		code, _, _ = runCLI(append(append([]string{}, base...), testAssetID, "--force")...)
		if code != 0 || read(t, "café brief.txt") != "hello" {
			t.Errorf("force failed: code %d", code)
		}
		if m, _ := filepath.Glob(".persadomapicli-*"); len(m) != 0 {
			t.Errorf("temp files left: %v", m)
		}
	})

	t.Run("out path", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "x.bin")
		code, _, errOut := runCLI(append(append([]string{}, base...), testAssetID, "--out", p)...)
		if code != 0 || read(t, p) != "hello" || !strings.Contains(errOut, p) {
			t.Fatalf("code %d stderr %q", code, errOut)
		}
		code, _, errOut = runCLI(append(append([]string{}, base...), testAssetID, "--out", p)...)
		if code != 1 || !strings.Contains(errOut, "already exists") {
			t.Errorf("code %d stderr %q", code, errOut)
		}
	})

	t.Run("out missing directory", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "no", "x.bin")
		code, _, errOut := runCLI(append(append([]string{}, base...), testAssetID, "--out", p)...)
		if code != 1 || !strings.Contains(errOut, "create file") {
			t.Errorf("code %d stderr %q", code, errOut)
		}
	})

	t.Run("stdout", func(t *testing.T) {
		code, out, errOut := runCLI(append(append([]string{}, base...), testAssetID, "--out", "-")...)
		if code != 0 || out != "hello" || strings.Contains(errOut, "Saved") {
			t.Errorf("code %d out %q stderr %q", code, out, errOut)
		}
	})

	t.Run("errors", func(t *testing.T) {
		code, _, errOut := runCLI(append(append([]string{}, base...), "bad")...)
		if code != 1 || !strings.Contains(errOut, "not an asset id") {
			t.Errorf("bad id: code %d stderr %q", code, errOut)
		}
		code, _, errOut = runCLI(append(append([]string{}, base...), "11111111-1111-1111-1111-111111111111")...)
		if code != 1 || !strings.Contains(errOut, "HTTP 404") || !strings.Contains(errOut, "not found or expired; list your assets with: persadomapicli asset list") {
			t.Errorf("404: code %d stderr %q", code, errOut)
		}
	})

	t.Run("edit", func(t *testing.T) {
		t.Chdir(t.TempDir())
		stubEditor(t, func(_ context.Context, _, text string, v editor.Validator) (string, error) {
			if !strings.Contains(text, "// ") {
				t.Errorf("template %q", text)
			}
			good := `{"id":"` + testAssetID + `"}`
			return good, v(good)
		})
		if code, _, errOut := runCLI(append(append([]string{}, base...), "-e")...); code != 0 {
			t.Errorf("code %d stderr %q", code, errOut)
		}
	})
	t.Run("edit canceled", func(t *testing.T) {
		stubEditor(t, func(context.Context, string, string, editor.Validator) (string, error) { return "", editor.ErrCanceled })
		if code, _, _ := runCLI(append(append([]string{}, base...), "-e")...); code != 1 {
			t.Errorf("code %d", code)
		}
	})
}

func TestDownloadName(t *testing.T) {
	id := testAssetID
	tests := []struct{ header, want string }{
		{"report.pdf", "report.pdf"},
		{"a/b/c%20d.txt", "c d.txt"},
		{`a\b.txt`, "b.txt"},
		{"%zz.txt", "%zz.txt"},
		{"", id},
		{"..", id},
		{"dir/", "dir"},
		{"/", id},
		{"a%00b", id},
	}
	for _, tc := range tests {
		if got := downloadName(tc.header, id); got != tc.want {
			t.Errorf("downloadName(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestSaveAtomicFailureLeavesNoFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := saveAtomic(filepath.Join(dir, "x"), failingReader{}); err == nil {
		t.Fatal("want error")
	}
	if m, _ := os.ReadDir(dir); len(m) != 0 {
		t.Errorf("left %d files", len(m))
	}
}

func TestRefuseExistingStatError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := refuseExisting(filepath.Join(f, "child")); err == nil || !strings.Contains(err.Error(), "check") {
		t.Errorf("err = %v", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, os.ErrClosed }
