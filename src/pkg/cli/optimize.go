package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

func newOptimizeCmd(g *globalFlags) *cobra.Command {
	var (
		acc                         accountFlags
		w                           waitFlags
		variant, context, brandFile string
		styleSet, assistantID, file string
		alwaysUse, exclude          []string
		charLimit, brandCharLimit   map[string]int
		seeding, edit               bool
	)
	cmd := &cobra.Command{
		Use:   "optimize",
		Short: "Generate scored, ranked alternatives for one variant (POST /v1/optimize)",
		Long: fmt.Sprintf(`Score one variant of marketing copy, generate alternatives for it and return them
scored and ranked.

Give the variant with --variant: plain text for the channel's text touchpoint,
or a JSON object of touchpoints for multi-part channels, such as
'{"headline":"...","body":"..."}'. Guide generation with --context, --always-use
(up to %d terms every alternative must contain), --char-limit and the brand
options: --brand-file (a JSON file holding the brand object), --style-set,
--exclude and --brand-char-limit. Where both limits name a touchpoint, the
smaller applies. Brand options shape generation only; they do not change scores.

Start from a JSON request with --file, or fill the request in the built-in
editor with --edit. Flags override the file.

The run is a job and can take minutes. By default the command waits for it and
prints the alternatives; --async prints the job id at once (read it later with
"persadomapicli job").`, api.MaxAlwaysUse),
		Example: `  persadomapicli optimize -a "Acme Bank" --variant "Your card is waiting" --context "0% intro APR for 15 months"
  persadomapicli optimize -a "Acme Bank" -c PUSH_NOTIFICATION --variant '{"headline":"Hi","body":"Apply today"}' --char-limit headline=35,body=100
  persadomapicli optimize -a "Acme Bank" --variant "Hello" --always-use "0% intro APR" --exclude "free money" --async
  persadomapicli optimize --edit`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := &api.OptimizeRequest{}
			if file != "" {
				if err := loadFile(cmd, file, req); err != nil {
					return err
				}
			}
			acc.apply(cmd, &req.Channel, &req.DisplayChannel, &req.Account)
			f := cmd.Flags()
			if f.Changed("variant") {
				t, err := parseVariant(variant)
				if err != nil {
					return err
				}
				req.Variant = t
			}
			if f.Changed("context") {
				req.Context = context
			}
			if f.Changed("always-use") {
				req.AlwaysUse = alwaysUse
			}
			if f.Changed("char-limit") {
				req.CharacterLimits = mergeLimits(req.CharacterLimits, charLimit)
			}
			if err := applyBrand(cmd, req, brandFile, styleSet, exclude, brandCharLimit); err != nil {
				return err
			}
			if f.Changed("assistant-id") || f.Changed("content-seeding") {
				if req.Generation == nil {
					req.Generation = &api.Generation{}
				}
				if f.Changed("assistant-id") {
					req.Generation.AssistantID = assistantID
				}
				if f.Changed("content-seeding") {
					req.Generation.ContentSeedingEnabled = &seeding
				}
			}
			if err := finalize(cmd, edit, "Optimize request (POST /v1/optimize)", req, optimizeTemplate); err != nil {
				return err
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			wait := api.MaxWaitSeconds
			if w.async {
				wait = 0
			}
			resp, err := c.Optimize(cmd.Context(), req, wait)
			if err != nil {
				return err
			}
			job, raw := &resp.Value, []byte(resp.Raw)
			if !w.async && !job.Done() {
				if job, raw, err = waitJob(cmd.Context(), c, job, w.timeout, cmd.ErrOrStderr()); err != nil {
					return err
				}
			}
			return printJob(cmd.OutOrStdout(), g.output, job, raw)
		},
	}
	acc.register(cmd)
	w.register(cmd, true)
	f := cmd.Flags()
	f.StringVarP(&variant, "variant", "v", "", "the variant to optimize: plain text, or a JSON object of touchpoints")
	f.StringVar(&context, "context", "", fmt.Sprintf("background that guides generation (up to %d characters, no personal data)", api.MaxContextLength))
	f.StringArrayVar(&alwaysUse, "always-use", nil, fmt.Sprintf("a term every alternative must contain (repeat for up to %d)", api.MaxAlwaysUse))
	f.StringToIntVar(&charLimit, "char-limit", nil, "maximum length per touchpoint, KEY=N (repeat, or comma-separated)")
	f.StringVar(&brandFile, "brand-file", "", "read the brand object (style guidelines, exclusions, limits) from this JSON file")
	f.StringVar(&styleSet, "style-set", "", "name of the style guideline set (brand.styleSetName)")
	f.StringArrayVar(&exclude, "exclude", nil, "a phrase the brand does not use (repeat; brand.exclusionList)")
	f.StringToIntVar(&brandCharLimit, "brand-char-limit", nil, "brand maximum length per touchpoint, KEY=N (repeat, or comma-separated)")
	f.StringVar(&assistantID, "assistant-id", "", "generation assistant id")
	f.BoolVar(&seeding, "content-seeding", false, "enable content seeding (--content-seeding=false disables it); sends the generation object")
	f.StringVarP(&file, "file", "f", "", "read the JSON request from this file (- for stdin)")
	f.BoolVarP(&edit, "edit", "e", false, "fill the request in the built-in editor, starting from the flags given")
	return apiCommand(cmd)
}

// mergeLimits adds the flag limits to dst, with lower-cased keys as the API expects.
func mergeLimits(dst, flag map[string]int) map[string]int {
	if dst == nil {
		dst = map[string]int{}
	}
	for k, v := range flag {
		dst[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return dst
}

// applyBrand builds req.Brand from the brand file, then the brand flags the user set.
func applyBrand(cmd *cobra.Command, req *api.OptimizeRequest, brandFile, styleSet string, exclude []string, limits map[string]int) error {
	f := cmd.Flags()
	if brandFile != "" {
		b := &api.Brand{}
		if err := loadFile(cmd, brandFile, b); err != nil {
			return err
		}
		req.Brand = b
	}
	if !f.Changed("style-set") && !f.Changed("exclude") && !f.Changed("brand-char-limit") {
		return nil
	}
	if req.Brand == nil {
		req.Brand = &api.Brand{}
	}
	if f.Changed("style-set") {
		req.Brand.StyleSetName = styleSet
	}
	if f.Changed("exclude") {
		req.Brand.ExclusionList = exclude
	}
	if f.Changed("brand-char-limit") {
		req.Brand.CharacterLimits = mergeLimits(req.Brand.CharacterLimits, limits)
	}
	return nil
}

func optimizeTemplate(r *api.OptimizeRequest) string {
	variant := r.Variant
	if len(variant) == 0 {
		variant = api.Touchpoints{"text": ""}
	}
	b := r.Brand
	if b == nil {
		b = &api.Brand{}
	}
	exclusions := b.ExclusionList
	if exclusions == nil {
		exclusions = []string{}
	}
	guidelines := b.StyleGuidelines
	if guidelines == nil {
		guidelines = map[string]json.RawMessage{}
	}
	brand := object{
		{"styleSetName", b.StyleSetName},
		{"exclusionList", exclusions},
		{"styleGuidelines", guidelines},
		{"characterLimits", orEmpty(b.CharacterLimits)},
	}
	if len(b.StyleGuidelinesPerTouchpoint) > 0 {
		brand = append(brand, field{"styleGuidelinesPerTouchpoint", b.StyleGuidelinesPerTouchpoint})
	}
	alwaysUse := r.AlwaysUse
	if alwaysUse == nil {
		alwaysUse = []string{}
	}
	help := append([]string{
		"Optimize one variant: the API scores it, generates alternatives and returns them scored and ranked.",
		"variant maps touchpoint keys to copy and is required, for example {\"text\": \"...\"}.",
		fmt.Sprintf("context: background that guides generation (up to %d characters, no personal data).", api.MaxContextLength),
		fmt.Sprintf("alwaysUse: up to %d terms (1 to %d characters each) every alternative must contain.", api.MaxAlwaysUse, api.MaxAlwaysUseLength),
		"characterLimits: maximum length per touchpoint, such as {\"headline\": 35}. Where brand.characterLimits names the same key, the smaller applies.",
		"brand shapes generation only and does not change scores. exclusionList: phrases the brand does not use.",
		"brand.styleGuidelines maps a section name to a list of styles, such as {\"Tone of Voice\": [{\"style\": \"Friendly\", \"instruction\": \"Warm, plain words\"}]}.",
		"A section may also be a plain string. brand.styleGuidelinesPerTouchpoint (optional) has the same shape per touchpoint key and wins for that touchpoint.",
		"generation is null unless set. When set it is {\"assistantId\": \"...\", \"contentSeedingEnabled\": true} and contentSeedingEnabled is required.",
	}, accountHelp()...)
	return template(help, object{
		{"channel", orDefault(r.Channel, api.DefaultChannel)},
		{"displayChannel", r.DisplayChannel},
		{"account", accountTemplate(r.Account)},
		{"variant", variant},
		{"context", r.Context},
		{"alwaysUse", alwaysUse},
		{"characterLimits", orEmpty(r.CharacterLimits)},
		{"brand", brand},
		{"generation", r.Generation},
	})
}

func orEmpty(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}
