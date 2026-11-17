package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/persado/martechapi-cli/src/pkg/editor"
	"github.com/spf13/cobra"
)

func TestGlobalFlagsValidate(t *testing.T) {
	tests := []struct {
		output  string
		wantErr bool
	}{
		{"text", false},
		{"json", false},
		{"yaml", true},
		{"", true},
	}
	for _, tc := range tests {
		t.Run(tc.output, func(t *testing.T) {
			err := (&globalFlags{output: tc.output}).validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validate(%q) err = %v, wantErr %v", tc.output, err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "--output must be text or json") {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestGlobalFlagsClient(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key")
	if err := os.WriteFile(keyFile, []byte("  pmk_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		g        globalFlags
		envKey   string
		envURL   string
		wantKey  string
		wantBase string
		wantLang string
		wantErr  string
	}{
		{name: "key from flag", g: globalFlags{apiKey: "pmk_flag"}, wantKey: "pmk_flag"},
		{name: "key from file is trimmed", g: globalFlags{apiKeyFile: keyFile}, wantKey: "pmk_file"},
		{name: "key from env is trimmed", envKey: " pmk_env \n", wantKey: "pmk_env"},
		{name: "flag beats file and env", g: globalFlags{apiKey: "pmk_flag", apiKeyFile: keyFile}, envKey: "pmk_env", wantKey: "pmk_flag"},
		{name: "file beats env", g: globalFlags{apiKeyFile: keyFile}, envKey: "pmk_env", wantKey: "pmk_file"},
		{name: "no key leaves it empty"},
		{name: "missing key file", g: globalFlags{apiKeyFile: filepath.Join(dir, "nope")}, wantErr: "read API key file"},
		{name: "base url from env", envURL: "https://env.example/api", wantBase: "https://env.example/api"},
		{name: "base url flag beats env", g: globalFlags{baseURL: "https://flag.example"}, envURL: "https://env.example/api", wantBase: "https://flag.example"},
		{name: "lang is passed on", g: globalFlags{lang: "fr"}, wantLang: "fr"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envAPIKey, tc.envKey)
			t.Setenv(envBaseURL, tc.envURL)
			c, err := tc.g.client()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.Key != tc.wantKey || c.BaseURL != tc.wantBase || c.Language != tc.wantLang {
				t.Errorf("client = key %q base %q lang %q, want %q %q %q", c.Key, c.BaseURL, c.Language, tc.wantKey, tc.wantBase, tc.wantLang)
			}
			if c.HTTP == nil || c.HTTP.Timeout <= 0 {
				t.Errorf("HTTP client must have a timeout, got %+v", c.HTTP)
			}
			if !strings.HasPrefix(c.UserAgent, "persadomapicli/") {
				t.Errorf("UserAgent = %q", c.UserAgent)
			}
		})
	}
}

func TestApiCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"nil annotations", &cobra.Command{Use: "a"}},
		{"existing annotations kept", &cobra.Command{Use: "b", Annotations: map[string]string{"x": "y"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := apiCommand(tc.cmd)
			if got != tc.cmd {
				t.Fatal("apiCommand must return its argument")
			}
			if _, ok := got.Annotations[annotationAPI]; !ok {
				t.Errorf("annotations = %v, want %q", got.Annotations, annotationAPI)
			}
			if tc.name == "existing annotations kept" && got.Annotations["x"] != "y" {
				t.Errorf("existing annotation lost: %v", got.Annotations)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "plain"))
	if err != nil {
		t.Fatal(err)
	}
	if isTerminal(f) {
		t.Error("a regular file is not a terminal")
	}
	_ = f.Close()
	if isTerminal(f) {
		t.Error("a closed file is not a terminal")
	}
}

func TestDefaultRunEditorNeedsTerminal(t *testing.T) {
	// Under go test stdin/stdout are not terminals.
	if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		t.Skip("running on an interactive terminal")
	}
	_, err := runEditor(context.Background(), "t", "x", func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeStrict(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"valid", `{"id":"a"}`, ""},
		{"unknown field", `{"id":"a","bogus":1}`, "unknown field"},
		{"trailing object", `{"id":"a"} {"id":"b"}`, "unexpected data after"},
		{"syntax error", `{"id":`, "invalid JSON"},
		{"wrong type", `{"id":1}`, "invalid JSON"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var r jobRequest
			err := decodeStrict([]byte(tc.in), &r)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(good, []byte(`{"id":"from-file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte(`{"nope":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		stdin   string
		want    string
		wantErr string
	}{
		{name: "file", path: good, want: "from-file"},
		{name: "stdin", path: "-", stdin: `{"id":"from-stdin"}`, want: "from-stdin"},
		{name: "stdin trailing data", path: "-", stdin: `{"id":"a"} x`, wantErr: "-: invalid JSON: unexpected data"},
		{name: "unknown field names the file", path: bad, wantErr: bad + ": invalid JSON"},
		{name: "missing file", path: filepath.Join(dir, "none"), wantErr: "read request"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetIn(strings.NewReader(tc.stdin))
			var r jobRequest
			err := loadFile(cmd, tc.path, &r)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if r.ID != tc.want {
				t.Errorf("ID = %q, want %q", r.ID, tc.want)
			}
		})
	}
}

func TestFinalize(t *testing.T) {
	const goodID = "job_0123456789abcdef0123456789abcdef"
	tmpl := func(r *jobRequest) string { return "// help\n" + r.ID }
	tests := []struct {
		name      string
		edit      bool
		id        string
		editor    func(ctx context.Context, title, text string, v editor.Validator) (string, error)
		wantID    string
		wantErr   string
		wantErrIs error
	}{
		{name: "no edit valid", id: goodID, wantID: goodID},
		{name: "no edit invalid", id: "bad", wantErr: "invalid request (use --edit to fix it interactively)"},
		{
			name: "edit success", edit: true, id: "start",
			editor: func(_ context.Context, title, text string, v editor.Validator) (string, error) {
				if title != "Title" || text != "// help\nstart" {
					t.Errorf("editor got title %q text %q", title, text)
				}
				out := `{"id":"` + goodID + `"}`
				return out, v(out)
			},
			wantID: goodID,
		},
		{
			name: "edit validator rejects invalid request", edit: true,
			editor: func(_ context.Context, _, _ string, v editor.Validator) (string, error) {
				err := v(`{"id":"bad"}`)
				if err == nil || !strings.Contains(err.Error(), "not a job id") {
					t.Errorf("validator err = %v", err)
				}
				return `{"id":"` + goodID + `"}`, nil
			},
			wantID: goodID,
		},
		{
			name: "edit validator rejects empty text", edit: true,
			editor: func(_ context.Context, _, _ string, v editor.Validator) (string, error) {
				if err := v(""); err == nil || !strings.Contains(err.Error(), "request is empty") {
					t.Errorf("validator err = %v", err)
				}
				return "", errors.New("stop")
			},
			wantErr: "stop",
		},
		{
			name: "edit validator rejects unknown field", edit: true,
			editor: func(_ context.Context, _, _ string, v editor.Validator) (string, error) {
				if err := v(`{"id":"x","extra":1}`); err == nil || !strings.Contains(err.Error(), "unknown field") {
					t.Errorf("validator err = %v", err)
				}
				return "", errors.New("stop")
			},
			wantErr: "stop",
		},
		{
			name: "edit canceled", edit: true,
			editor: func(context.Context, string, string, editor.Validator) (string, error) {
				return "", editor.ErrCanceled
			},
			wantErrIs: editor.ErrCanceled,
		},
		{
			name: "edit returns undecodable text", edit: true,
			editor: func(context.Context, string, string, editor.Validator) (string, error) {
				return "not json", nil
			},
			wantErr: "invalid JSON",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.editor != nil {
				stubEditor(t, tc.editor)
			} else {
				stubEditor(t, func(context.Context, string, string, editor.Validator) (string, error) {
					t.Error("editor must not run without --edit")
					return "", nil
				})
			}
			req := &jobRequest{ID: tc.id}
			err := finalize(&cobra.Command{}, tc.edit, "Title", req, tmpl)
			switch {
			case tc.wantErrIs != nil:
				if !errors.Is(err, tc.wantErrIs) {
					t.Fatalf("err = %v, want %v", err, tc.wantErrIs)
				}
			case tc.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if req.ID != tc.wantID {
					t.Errorf("ID = %q, want %q", req.ID, tc.wantID)
				}
			}
		})
	}
}

func TestObjectKeepsOrder(t *testing.T) {
	o := object{{"zeta", 1}, {"alpha", "a"}, {"mid", []int{1, 2}}, {"nested", object{{"b", true}, {"a", nil}}}}
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"zeta":1,"alpha":"a","mid":[1,2],"nested":{"b":true,"a":null}}`
	if string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
	if b, _ := json.Marshal(object{}); string(b) != "{}" {
		t.Errorf("empty object = %s", b)
	}
	if _, err := json.Marshal(object{{"bad", make(chan int)}}); err == nil {
		t.Error("expected an error for an unmarshalable value")
	}
}

func TestTemplate(t *testing.T) {
	got := template([]string{"first", "second"}, object{{"b", 1}, {"a", "x"}})
	want := "// first\n// second\n{\n  \"b\": 1,\n  \"a\": \"x\"\n}\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := template(nil, object{{"k", 1}}); strings.HasPrefix(got, "//") {
		t.Errorf("no help should give no comments: %q", got)
	}
}

func TestAccountTemplate(t *testing.T) {
	tests := []struct {
		name string
		in   api.Account
		want string
	}{
		{"empty", api.Account{}, `{"name":"","industry":"","country":"","region":"","language":""}`},
		{"full with parent", api.Account{Name: "Acme", Industry: "RETAIL", Country: "US", Region: "r", Language: "en", ParentID: 7},
			`{"name":"Acme","industry":"RETAIL","country":"US","region":"r","language":"en","parentId":7}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(accountTemplate(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tc.want {
				t.Errorf("got %s, want %s", b, tc.want)
			}
		})
	}
}

func TestAccountHelp(t *testing.T) {
	joined := strings.Join(accountHelp(), "\n")
	for _, want := range []string{"channel: EMAIL_SUBJECT", "default " + api.DefaultChannel, "account.name is required", "PUSH_NOTIFICATION=headline|body", "account.industry"} {
		if !strings.Contains(joined, want) {
			t.Errorf("help does not contain %q:\n%s", want, joined)
		}
	}
	for _, c := range api.Channels {
		if !strings.Contains(touchpointHelp(), c+"=") {
			t.Errorf("touchpointHelp lacks channel %s", c)
		}
	}
}

func TestParseVariant(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    api.Touchpoints
		wantErr string
	}{
		{"plain text", "Hello world", api.Touchpoints{"text": "Hello world"}, ""},
		{"plain text keeps spaces", "  Hi ", api.Touchpoints{"text": "  Hi "}, ""},
		{"json object", ` {"headline":"H","body":"B"}`, api.Touchpoints{"headline": "H", "body": "B"}, ""},
		{"bad json", `{"headline":`, nil, "--variant"},
		{"non string value", `{"headline":1}`, nil, "--variant"},
		{"trailing data", `{"headline":"H"} x`, nil, "unexpected data"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseVariant(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("got[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestAccountFlagsApply(t *testing.T) {
	newCmd := func() (*cobra.Command, *accountFlags) {
		cmd := &cobra.Command{Use: "x"}
		var a accountFlags
		a.register(cmd)
		return cmd, &a
	}
	t.Run("only changed flags are applied and enums normalised", func(t *testing.T) {
		cmd, a := newCmd()
		if err := cmd.ParseFlags([]string{"-c", "sms_ems", "--display-channel", "tik_tok", "-a", "Acme", "--industry", "retail",
			"--country", "gr", "--region", "Attica", "--language", "EL", "--parent-id", "42"}); err != nil {
			t.Fatal(err)
		}
		channel, display := "KEEP", "KEEP"
		acc := api.Account{Name: "old"}
		a.apply(cmd, &channel, &display, &acc)
		want := api.Account{Name: "Acme", Industry: "RETAIL", Country: "GR", Region: "Attica", Language: "el", ParentID: 42}
		if channel != "SMS_EMS" || display != "TIK_TOK" || acc != want {
			t.Errorf("got %q %q %+v", channel, display, acc)
		}
	})
	t.Run("unchanged flags keep the request values", func(t *testing.T) {
		cmd, a := newCmd()
		if err := cmd.ParseFlags([]string{"--region", "R"}); err != nil {
			t.Fatal(err)
		}
		channel, display := "WEB_PAGE", "BILLBOARD"
		acc := api.Account{Name: "Acme", Industry: "X", Country: "US", Language: "en", ParentID: 3}
		a.apply(cmd, &channel, &display, &acc)
		want := api.Account{Name: "Acme", Industry: "X", Country: "US", Region: "R", Language: "en", ParentID: 3}
		if channel != "WEB_PAGE" || display != "BILLBOARD" || acc != want {
			t.Errorf("got %q %q %+v", channel, display, acc)
		}
	})
	t.Run("explicit empty value clears the field", func(t *testing.T) {
		cmd, a := newCmd()
		if err := cmd.ParseFlags([]string{"--industry", "", "--parent-id", "0"}); err != nil {
			t.Fatal(err)
		}
		var channel, display string
		acc := api.Account{Industry: "X", ParentID: 5}
		a.apply(cmd, &channel, &display, &acc)
		if acc.Industry != "" || acc.ParentID != 0 {
			t.Errorf("acc = %+v", acc)
		}
	})
}

func TestPrintJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"indents", `{"a":1,"b":[2]}`, "{\n  \"a\": 1,\n  \"b\": [\n    2\n  ]\n}\n"},
		{"invalid json is written as is", `not json`, "not json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := printJSON(&b, []byte(tc.in)); err != nil {
				t.Fatal(err)
			}
			if b.String() != tc.want {
				t.Errorf("got %q, want %q", b.String(), tc.want)
			}
		})
	}
}

func TestOrDash(t *testing.T) {
	n, s := 5, "hi"
	tests := []struct{ got, want string }{
		{orDash[int](nil, "%d"), "-"},
		{orDash(&n, "%d"), "5"},
		{orDash(&n, "%d/5"), "5/5"},
		{orDash(&s, "%s"), "hi"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

func TestWithFlagHints(t *testing.T) {
	plain := errors.New("plain")
	if got := withFlagHints(plain); got != plain {
		t.Errorf("non-validation error changed: %v", got)
	}
	in := api.ValidationError{
		{Field: "account.name", Reason: "is required"},
		{Field: "account.industry", Reason: "bad"},
		{Field: "channel", Reason: "bad"},
		{Field: "variants[1].text", Reason: "bad"},
		{Field: "variants", Reason: "bad"},
		{Field: "variant", Reason: "bad"},
		{Field: "other", Reason: "bad"},
		{Field: "channelX", Reason: "bad"},
	}
	want := []string{
		"account.name (--account/-a)", "account.industry (--industry)", "channel (--channel/-c)",
		"variants[1].text (--variant/-v)", "variants (--variant/-v)", "variant (--variant/-v)", "other", "channelX",
	}
	got := withFlagHints(in).(api.ValidationError)
	for i, w := range want {
		if got[i].Field != w {
			t.Errorf("field %d = %q, want %q", i, got[i].Field, w)
		}
	}
	if in[0].Field != "account.name" {
		t.Error("input was modified")
	}
}

func TestFinalizeHintsFlags(t *testing.T) {
	req := &api.ScoreRequest{}
	err := finalize(&cobra.Command{}, false, "T", req, func(*api.ScoreRequest) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "account.name (--account/-a): is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseVariants(t *testing.T) {
	got, err := parseVariants([]string{"hello", `{"headline":"h","body":"b"}`})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0]["text"] != "hello" || got[1]["headline"] != "h" {
		t.Errorf("got %v", got)
	}
	if got, err := parseVariants(nil); err != nil || len(got) != 0 {
		t.Errorf("empty: %v, %v", got, err)
	}
	if _, err := parseVariants([]string{"ok", `{"x":`}); err == nil || !strings.Contains(err.Error(), "--variant") {
		t.Errorf("err = %v", err)
	}
}

func TestVariantsTemplate(t *testing.T) {
	if got := variantsTemplate(nil); len(got) != 1 || got[0].(api.Touchpoints)["text"] != "" {
		t.Errorf("default = %v", got)
	}
	got := variantsTemplate([]api.Touchpoints{{"text": "a"}, {"text": "b"}})
	if len(got) != 2 || got[1].(api.Touchpoints)["text"] != "b" {
		t.Errorf("got %v", got)
	}
}

func TestOrDefault(t *testing.T) {
	tests := []struct{ v, def, want string }{{"", "d", "d"}, {"x", "d", "x"}, {"", "", ""}}
	for _, tc := range tests {
		if got := orDefault(tc.v, tc.def); got != tc.want {
			t.Errorf("orDefault(%q,%q) = %q, want %q", tc.v, tc.def, got, tc.want)
		}
	}
}

func TestOneLine(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a\nb", "a / b"},
		{"  a   b  ", "a b"},
		{"a\n\nb", "a / / b"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := oneLine(tc.in); got != tc.want {
			t.Errorf("oneLine(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
