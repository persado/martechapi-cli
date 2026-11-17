package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/persado/martechapi-cli/src/pkg/editor"
	"github.com/persado/martechapi-cli/src/pkg/version"
	"github.com/spf13/cobra"
)

// Environment variables read for the persistent flags.
const (
	envAPIKey  = "PERSADO_API_KEY"
	envBaseURL = "PERSADO_API_URL"
)

// httpTimeout bounds one HTTP exchange: more than the 50 s server wait plus a large transfer.
const httpTimeout = 2 * time.Minute

const (
	outputText = "text"
	outputJSON = "json"
)

// globalFlags are the persistent flags shared by every API command.
type globalFlags struct {
	apiKey     string
	apiKeyFile string
	baseURL    string
	lang       string
	output     string
}

func (g *globalFlags) register(cmd *cobra.Command) {
	f := cmd.PersistentFlags()
	f.StringVar(&g.apiKey, "api-key", "", "API key (pmk_...); default $"+envAPIKey)
	f.StringVar(&g.apiKeyFile, "api-key-file", "", "read the API key from this file")
	f.StringVar(&g.baseURL, "base-url", "", "API base URL; default $"+envBaseURL+" or "+api.DefaultBaseURL)
	f.StringVar(&g.lang, "lang", "", "preferred language of localised text (Accept-Language), such as en or fr")
	f.StringVarP(&g.output, "output", "o", outputText, "output format: text or json")
}

func (g *globalFlags) validate() error {
	if g.output != outputText && g.output != outputJSON {
		return fmt.Errorf("--output must be text or json, not %q", g.output)
	}
	return nil
}

func (g *globalFlags) client() (*api.Client, error) {
	key := g.apiKey
	if key == "" && g.apiKeyFile != "" {
		b, err := os.ReadFile(g.apiKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read API key file: %w", err)
		}
		key = strings.TrimSpace(string(b))
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv(envAPIKey))
	}
	base := g.baseURL
	if base == "" {
		base = os.Getenv(envBaseURL)
	}
	return &api.Client{
		HTTP:      &http.Client{Timeout: httpTimeout},
		BaseURL:   base,
		Key:       key,
		Language:  g.lang,
		UserAgent: "persadomapicli/" + version.Version,
	}, nil
}

// apiCommand marks cmd as calling the Martech API, so the root prints the banner.
func apiCommand(cmd *cobra.Command) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annotationAPI] = ""
	return cmd
}

// isTerminal reports whether f is an interactive terminal.
func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// runEditor opens the built-in editor. Tests replace it.
var runEditor = func(ctx context.Context, title, text string, v editor.Validator) (string, error) {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return "", errors.New("--edit needs an interactive terminal")
	}
	return editor.Edit(ctx, title, text, v, os.Stdin, os.Stdout)
}

// validator is a request that checks itself.
type validator interface{ Validate() error }

// decodeStrict decodes one JSON value into v, rejecting unknown fields and trailing data.
func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return errors.New("invalid JSON: unexpected data after the request object")
	}
	return nil
}

// loadFile decodes the JSON request in path ("-" reads stdin) into req.
func loadFile(cmd *cobra.Command, path string, req any) error {
	var (
		data []byte
		err  error
	)
	if path == "-" {
		data, err = io.ReadAll(cmd.InOrStdin())
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	if err := decodeStrict(data, req); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// finalize validates req, or with edit opens the editor on template(req)
// until the user saves a request that validates, and stores it in req.
func finalize[T any, P interface {
	*T
	validator
}](cmd *cobra.Command, edit bool, title string, req P, template func(P) string) error {
	if !edit {
		if err := req.Validate(); err != nil {
			return fmt.Errorf("invalid request (use --edit to fix it interactively):\n%w", withFlagHints(err))
		}
		return nil
	}
	check := func(text string) error {
		if text == "" {
			return errors.New("the request is empty")
		}
		var next T
		if err := decodeStrict([]byte(text), &next); err != nil {
			return err
		}
		return P(&next).Validate()
	}
	text, err := runEditor(cmd.Context(), title, template(req), check)
	if err != nil {
		return err
	}
	var next T
	if err := decodeStrict([]byte(text), &next); err != nil {
		return err
	}
	*req = next
	return nil
}

// fieldFlags maps request fields to the flags that set them.
var fieldFlags = []struct{ field, flag string }{
	{"account.name", "--account/-a"},
	{"account.industry", "--industry"},
	{"account.country", "--country"},
	{"account.region", "--region"},
	{"account.language", "--language"},
	{"account.parentId", "--parent-id"},
	{"channel", "--channel/-c"},
	{"displayChannel", "--display-channel"},
	{"variants", "--variant/-v"},
	{"variant", "--variant/-v"},
}

// withFlagHints names the flag after each field of a validation error, so a
// user who never saw the JSON request knows what to change.
func withFlagHints(err error) error {
	var ve api.ValidationError
	if !errors.As(err, &ve) {
		return err
	}
	out := make(api.ValidationError, len(ve))
	for i, d := range ve {
		out[i] = d
		for _, ff := range fieldFlags {
			rest, ok := strings.CutPrefix(d.Field, ff.field)
			if ok && (rest == "" || rest[0] == '.' || rest[0] == '[') {
				out[i].Field = d.Field + " (" + ff.flag + ")"
				break
			}
		}
	}
	return out
}

// field is one key of an ordered JSON object, for templates.
type field struct {
	key string
	val any
}

// object is a JSON object that keeps its key order when marshalled.
type object []field

func (o object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, f := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(f.key)
		v, err := json.Marshal(f.val)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// template renders help lines as // comments followed by body as indented JSON.
func template(help []string, body any) string {
	var b strings.Builder
	for _, h := range help {
		b.WriteString("// " + h + "\n")
	}
	j, _ := json.MarshalIndent(body, "", "  ")
	b.Write(j)
	b.WriteByte('\n')
	return b.String()
}

// accountTemplate is the account object of a request template.
func accountTemplate(a api.Account) object {
	o := object{
		{"name", a.Name},
		{"industry", a.Industry},
		{"country", a.Country},
		{"region", a.Region},
		{"language", a.Language},
	}
	if a.ParentID != 0 {
		o = append(o, field{"parentId", a.ParentID})
	}
	return o
}

// accountHelp documents the account and channel fields in a template.
func accountHelp() []string {
	return []string{
		"channel: " + strings.Join(api.Channels, ", ") + " (default " + api.DefaultChannel + ")",
		"displayChannel (optional label of the real medium, no effect on scores), such as TIK_TOK or EMAIL_BODY",
		"account.name is required: the brand the copy is for",
		"account.industry (optional): " + strings.Join(api.Industries, ", "),
		"account.country: two capitals such as US, GR (default US); account.language: such as en, fr (default from country)",
		"Touchpoint keys per channel: " + touchpointHelp(),
	}
}

func touchpointHelp() string {
	parts := make([]string, 0, len(api.Channels))
	for _, c := range api.Channels {
		parts = append(parts, c+"="+strings.Join(api.ChannelTouchpoints[c], "|"))
	}
	return strings.Join(parts, "; ")
}

// parseVariant reads a --variant value: a JSON object of touchpoints, or plain
// text for the channel's text touchpoint.
func parseVariant(s string) (api.Touchpoints, error) {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "{") {
		var v api.Touchpoints
		if err := decodeStrict([]byte(t), &v); err != nil {
			return nil, fmt.Errorf("--variant %s: %w", s, err)
		}
		return v, nil
	}
	return api.Touchpoints{"text": s}, nil
}

// parseVariants reads every --variant value.
func parseVariants(vs []string) ([]api.Touchpoints, error) {
	out := make([]api.Touchpoints, 0, len(vs))
	for _, v := range vs {
		t, err := parseVariant(v)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// variantsTemplate is the variants array of a request template: the variants
// given, or one empty text variant to fill in.
func variantsTemplate(vs []api.Touchpoints) []any {
	out := make([]any, 0, len(vs))
	for _, v := range vs {
		out = append(out, v)
	}
	if len(out) == 0 {
		out = append(out, api.Touchpoints{"text": ""})
	}
	return out
}

// accountFlags are the flags that fill the account and channel fields.
type accountFlags struct {
	channel, displayChannel string
	account                 api.Account
}

func (a *accountFlags) register(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVarP(&a.channel, "channel", "c", "", "channel: "+strings.Join(api.Channels, ", ")+" (default "+api.DefaultChannel+")")
	f.StringVar(&a.displayChannel, "display-channel", "", "label of the real medium, such as TIK_TOK (no effect on scores)")
	f.StringVarP(&a.account.Name, "account", "a", "", "brand the copy is for (account.name, required)")
	f.StringVar(&a.account.Industry, "industry", "", "industry: "+strings.Join(api.Industries, ", "))
	f.StringVar(&a.account.Country, "country", "", "two-letter country, such as US or GR (default US)")
	f.StringVar(&a.account.Region, "region", "", "region of the account")
	f.StringVar(&a.account.Language, "language", "", "language code without region, such as en or fr (default from country)")
	f.IntVar(&a.account.ParentID, "parent-id", 0, "parent account id")
}

// apply copies every flag the user set onto the request fields.
func (a *accountFlags) apply(cmd *cobra.Command, channel, display *string, acc *api.Account) {
	f := cmd.Flags()
	set := func(name string, dst *string, v string) {
		if f.Changed(name) {
			*dst = v
		}
	}
	set("channel", channel, strings.ToUpper(a.channel))
	set("display-channel", display, strings.ToUpper(a.displayChannel))
	set("account", &acc.Name, a.account.Name)
	set("industry", &acc.Industry, strings.ToUpper(a.account.Industry))
	set("country", &acc.Country, strings.ToUpper(a.account.Country))
	set("region", &acc.Region, a.account.Region)
	set("language", &acc.Language, strings.ToLower(a.account.Language))
	if f.Changed("parent-id") {
		acc.ParentID = a.account.ParentID
	}
}

// printJSON writes raw indented.
func printJSON(w io.Writer, raw []byte) error {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		_, err = w.Write(raw)
		return err
	}
	out.WriteByte('\n')
	_, err := w.Write(out.Bytes())
	return err
}

// orDash formats a possibly missing value.
func orDash[T any](v *T, format string) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf(format, *v)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// oneLine joins the lines of s with " / " so a table row stays on one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " / ")), " ")
}
