package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/persado/martechapi-cli/src/pkg/editor"
)

type gotUpload struct {
	meta api.AssetMetadata
	file string
	auth string
}

// assetServer answers POST /v1/asset/create, failing for file names in failNames.
func assetServer(t *testing.T, failNames ...string) (*httptest.Server, *[]gotUpload) {
	t.Helper()
	var (
		mu   sync.Mutex
		got  []gotUpload
		seq  int
		fail = map[string]bool{}
	)
	for _, n := range failNames {
		fail[n] = true
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/asset/create" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("content type: %v", err)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		var u gotUpload
		u.auth = r.Header.Get("Authorization")
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			b, _ := io.ReadAll(p)
			switch p.FormName() {
			case "metadata":
				if err := json.Unmarshal(b, &u.meta); err != nil {
					t.Errorf("metadata: %v", err)
				}
			case "file":
				u.file = string(b)
			}
		}
		mu.Lock()
		got = append(got, u)
		seq++
		n := seq
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if fail[u.meta.Filename] {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"rejected"}}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(api.Asset{ID: strings.Repeat("a", 35) + string(rune('0'+n)), Filename: u.meta.Filename,
			MimeType: u.meta.MimeType, Size: u.meta.Length, Version: 1, ExpiresAt: "2030-01-01T00:00:00Z"})
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func assetWriteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAssetCreateUploads(t *testing.T) {
	dir := t.TempDir()
	pdf := assetWriteFile(t, dir, "a.pdf", "%PDF-1")
	bin := assetWriteFile(t, dir, "noext", "plain words here")
	srv, got := assetServer(t)
	code, out, errs := runCLI("asset", "create", "--base-url", srv.URL, "--api-key", "pmk_test",
		"--expires-in-days", "7", "--owner", "me@x.com", "--attr", "k=v=w", pdf, bin)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	if len(*got) != 2 {
		t.Fatalf("uploads: %d", len(*got))
	}
	u := (*got)[0]
	if u.meta.Filename != "a.pdf" || u.meta.MimeType != "application/pdf" || u.meta.Length != 6 || u.file != "%PDF-1" ||
		u.meta.ExpiresInDays != 7 || u.meta.Owner != "me@x.com" || u.meta.Attributes["k"] != "v=w" || u.auth != "Bearer pmk_test" {
		t.Errorf("first upload: %+v", u)
	}
	if m := (*got)[1].meta.MimeType; !strings.HasPrefix(m, "text/plain") {
		t.Errorf("sniffed type %q", m)
	}
	if !strings.HasPrefix(out, "ID") || strings.Count(out, "\n") != 3 || !strings.Contains(out, "2030-01-01T00:00:00Z") {
		t.Errorf("stdout %q", out)
	}
}

func TestAssetCreateJSONAndAlias(t *testing.T) {
	dir := t.TempDir()
	p := assetWriteFile(t, dir, "a.txt", "hi")
	srv, got := assetServer(t)
	code, out, errs := runCLI("asset", "upload", "--base-url", srv.URL, "--api-key", "k", "-o", "json",
		"--name", "x/y.txt", "--mime-type", "text/csv", p)
	if code != 0 {
		t.Fatalf("exit %d %q", code, errs)
	}
	var arr []api.Asset
	if err := json.Unmarshal([]byte(out), &arr); err != nil || len(arr) != 1 || arr[0].Filename != "x/y.txt" {
		t.Errorf("out %q err %v", out, err)
	}
	if m := (*got)[0].meta; m.Filename != "x/y.txt" || m.MimeType != "text/csv" {
		t.Errorf("meta %+v", m)
	}
}

func TestAssetCreateContinuesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	a := assetWriteFile(t, dir, "a.txt", "1")
	b := assetWriteFile(t, dir, "b.txt", "2")
	c := assetWriteFile(t, dir, "c.txt", "3")
	srv, got := assetServer(t, filepath.Base(b))
	for _, o := range []string{"text", "json"} {
		*got = nil
		code, out, errs := runCLI("asset", "create", "--base-url", srv.URL, "--api-key", "k", "-o", o, a, b, c)
		if code != 1 || len(*got) != 3 {
			t.Errorf("%s: exit %d uploads %d", o, code, len(*got))
		}
		if !strings.Contains(errs, b+":") || !strings.Contains(errs, "1 of 3 uploads failed") {
			t.Errorf("%s: stderr %q", o, errs)
		}
		if o == "json" {
			var arr []api.Asset
			if err := json.Unmarshal([]byte(out), &arr); err != nil || len(arr) != 2 {
				t.Errorf("json out %q", out)
			}
		}
	}
}

func TestAssetCreateRejectsBeforeUpload(t *testing.T) {
	dir := t.TempDir()
	ok := assetWriteFile(t, dir, "ok.txt", "x")
	empty := assetWriteFile(t, dir, "empty.txt", "")
	big := assetWriteFile(t, dir, "big.txt", strings.Repeat("x", assetMaxBytes+1))
	srv, got := assetServer(t)
	base := []string{"asset", "create", "--base-url", srv.URL, "--api-key", "k"}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"directory", []string{dir}, "is a directory"},
		{"empty", []string{ok, empty}, "is empty"},
		{"missing", []string{filepath.Join(dir, "nope")}, "no such file"},
		{"too big", []string{ok, big}, "the limit is"},
		{"bad days", []string{"--expires-in-days", "91", ok}, "expiresInDays"},
		{"bad owner", []string{"--owner", "nobody", ok}, "owner"},
		{"bad attr", []string{"--attr", "novalue", ok}, "must be KEY=VALUE"},
		{"empty attr key", []string{"--attr", "=v", ok}, "must be KEY=VALUE"},
		{"name with two files", []string{"--name", "n", ok, ok}, "one file only"},
		{"no files", nil, "requires at least 1 arg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errs := runCLI(append(append([]string{}, base...), tc.args...)...)
			if code != 1 || out != "" || !strings.Contains(errs, tc.want) {
				t.Errorf("exit %d out %q stderr %q", code, out, errs)
			}
		})
	}
	if len(*got) != 0 {
		t.Errorf("%d uploads sent", len(*got))
	}
}

func TestAssetCreateEdit(t *testing.T) {
	dir := t.TempDir()
	one := assetWriteFile(t, dir, "a.pdf", "x")
	two := assetWriteFile(t, dir, "b.pdf", "y")
	srv, got := assetServer(t)
	base := []string{"asset", "create", "--base-url", srv.URL, "--api-key", "k", "--edit"}

	t.Run("single file success", func(t *testing.T) {
		*got = nil
		stubEditor(t, func(_ context.Context, _, text string, v editor.Validator) (string, error) {
			for _, s := range []string{`"filename": "` + filepath.Base(one) + `"`, `"mimeType": "application/pdf"`, "expiresInDays", "// "} {
				if !strings.Contains(text, s) {
					t.Errorf("template lacks %q:\n%s", s, text)
				}
			}
			edited := `{"filename":"renamed.pdf","mimeType":"application/pdf","expiresInDays":3,"attributes":{"a":"b"}}`
			return edited, v(edited)
		})
		if code, _, errs := runCLI(append(base, one)...); code != 0 {
			t.Fatalf("exit %d %q", code, errs)
		}
		m := (*got)[0].meta
		if m.Filename != "renamed.pdf" || m.ExpiresInDays != 3 || m.Attributes["a"] != "b" {
			t.Errorf("meta %+v", m)
		}
	})
	t.Run("multiple files omit filename", func(t *testing.T) {
		*got = nil
		stubEditor(t, func(_ context.Context, _, text string, v editor.Validator) (string, error) {
			if strings.Contains(text, `"filename"`) {
				t.Errorf("template has a filename:\n%s", text)
			}
			edited := `{"owner":"me@x.com","mimeType":"text/csv"}`
			return edited, v(edited)
		})
		if code, _, errs := runCLI(append(base, one, two)...); code != 0 {
			t.Fatalf("exit %d %q", code, errs)
		}
		if len(*got) != 2 || (*got)[1].meta.Owner != "me@x.com" || (*got)[0].meta.MimeType != "text/csv" || (*got)[1].meta.Filename != filepath.Base(two) {
			t.Errorf("uploads %+v", *got)
		}
	})
	t.Run("filename for several files", func(t *testing.T) {
		stubEditor(t, func(context.Context, string, string, editor.Validator) (string, error) {
			return `{"filename":"x"}`, nil
		})
		code, _, errs := runCLI(append(base, one, two)...)
		if code != 1 || !strings.Contains(errs, "one file only") {
			t.Errorf("exit %d %q", code, errs)
		}
	})
	t.Run("validation error", func(t *testing.T) {
		stubEditor(t, func(context.Context, string, string, editor.Validator) (string, error) {
			return "", api.ValidationError{{Field: "owner", Reason: "must be an e-mail address"}}
		})
		code, _, errs := runCLI(append(base, one)...)
		if code != 1 || !strings.Contains(errs, "owner") {
			t.Errorf("exit %d %q", code, errs)
		}
	})
	t.Run("validator rejects", func(t *testing.T) {
		stubEditor(t, func(_ context.Context, _, _ string, v editor.Validator) (string, error) {
			if err := v(`{"expiresInDays":99}`); err == nil {
				t.Error("expected validation error")
			}
			if err := v(""); err == nil {
				t.Error("expected error for empty text")
			}
			return "", v(`{"expiresInDays":99}`)
		})
		if code, _, _ := runCLI(append(base, one)...); code != 1 {
			t.Errorf("exit %d", code)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		stubEditor(t, func(context.Context, string, string, editor.Validator) (string, error) { return "", editor.ErrCanceled })
		code, _, errs := runCLI(append(base, one)...)
		if code != 1 || errs == "" {
			t.Errorf("exit %d %q", code, errs)
		}
	})
}

func TestAssetCreateNoKey(t *testing.T) {
	t.Setenv(envAPIKey, "")
	dir := t.TempDir()
	p := assetWriteFile(t, dir, "a.txt", "x")
	srv, _ := assetServer(t)
	code, _, errs := runCLI("asset", "create", "--base-url", srv.URL, "--api-key-file", filepath.Join(dir, "nokey"), p)
	if code != 1 || !strings.Contains(errs, "API key file") {
		t.Errorf("exit %d %q", code, errs)
	}
}

func TestAssetCreateRejectsUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("device files and file modes differ on Windows")
	}
	dir := t.TempDir()
	locked := assetWriteFile(t, dir, "locked", "data")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
	tests := []struct {
		name, path, want string
		skip             bool
	}{
		{"device", os.DevNull, "is not a regular file", false},
		{"unreadable", locked, "locked", os.Geteuid() == 0},
	}
	srv, got := assetServer(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip("permissions are not enforced for root")
			}
			code, out, errs := runCLI("asset", "create", "--base-url", srv.URL, "--api-key", "k", tc.path)
			if code != 1 || out != "" || !strings.Contains(errs, "cannot upload") || !strings.Contains(errs, tc.want) {
				t.Errorf("exit %d out %q stderr %q", code, out, errs)
			}
		})
	}
	if len(*got) != 0 {
		t.Errorf("%d uploads sent", len(*got))
	}
}

func TestDetectMime(t *testing.T) {
	dir := t.TempDir()
	if _, err := detectMime(filepath.Join(dir, "missing")); err == nil {
		t.Error("expected error for a missing file")
	}
	if m, _ := detectMime(assetWriteFile(t, dir, "x.json", "{}")); !strings.HasPrefix(m, "application/json") {
		t.Errorf("json: %q", m)
	}
	if m, _ := detectMime(assetWriteFile(t, dir, "blob", "\x89PNG\r\n\x1a\n")); m != "image/png" {
		t.Errorf("png: %q", m)
	}
}

func TestUploadOneMissingFile(t *testing.T) {
	_, err := uploadOne(nil, nil, assetFile{path: "/no/such/file"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestStoredName(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "brief.pdf")
	tests := []struct{ in, want string }{
		{"brief.pdf", "brief.pdf"},
		{"docs/brief.pdf", "docs/brief.pdf"},
		{abs, "brief.pdf"},
		{"../up/brief.pdf", "brief.pdf"},
		{"a/../../brief.pdf", "brief.pdf"},
	}
	if filepath.Separator == '\\' {
		tests = append(tests, struct{ in, want string }{`docs\brief.pdf`, "docs/brief.pdf"})
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := storedName(tt.in); got != tt.want {
				t.Errorf("storedName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUploadAssetsStopsWhenCanceled(t *testing.T) {
	dir := t.TempDir()
	a := assetWriteFile(t, dir, "a.txt", "1")
	b := assetWriteFile(t, dir, "b.txt", "2")
	srv, got := assetServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"asset", "create", "--base-url", srv.URL, "--api-key", "k", a, b}, &stdout, &stderr)
	if code != 1 || len(*got) != 0 || strings.Count(stderr.String(), "canceled") != 1 {
		t.Errorf("exit %d uploads %d stderr %q", code, len(*got), stderr.String())
	}
}
