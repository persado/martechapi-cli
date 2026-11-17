package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

func newPerformanceCmd(g *globalFlags) *cobra.Command {
	var (
		acc      accountFlags
		variants []string
		file     string
		edit     bool
	)
	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Predict the performance of 1 to 10 variants (POST /v1/performance)",
		Long: fmt.Sprintf(`Predict the performance of 1 to %d variants of marketing copy. The call is
synchronous: the predictions are printed when it returns. A variant the model
cannot predict shows "-".

Give each variant with --variant: plain text for the channel's text touchpoint,
or a JSON object of touchpoints for multi-part channels, such as
'{"headline":"...","body":"..."}'. Start from a JSON request with --file, or
fill the request in the built-in editor with --edit. Flags override the file.`, api.MaxPerformanceVariants),
		Example: `  persadomapicli performance -a "Acme Bank" --variant "Your card is waiting" --variant "Apply in minutes"
  persadomapicli performance --file request.json -o json
  persadomapicli performance --edit`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := &api.PerformanceRequest{}
			if file != "" {
				if err := loadFile(cmd, file, req); err != nil {
					return err
				}
			}
			acc.apply(cmd, &req.Channel, &req.DisplayChannel, &req.Account)
			if cmd.Flags().Changed("variant") {
				req.Variants = nil
				for _, v := range variants {
					t, err := parseVariant(v)
					if err != nil {
						return err
					}
					req.Variants = append(req.Variants, t)
				}
			}
			if err := finalize(cmd, edit, "Performance request (POST /v1/performance)", req, performanceTemplate); err != nil {
				return err
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			resp, err := c.Performance(cmd.Context(), req)
			if err != nil {
				return err
			}
			if g.output == outputJSON {
				return printJSON(cmd.OutOrStdout(), []byte(resp.Raw))
			}
			return renderPerformance(cmd.OutOrStdout(), &resp.Value)
		},
	}
	acc.register(cmd)
	f := cmd.Flags()
	f.StringArrayVarP(&variants, "variant", "v", nil, "a variant: plain text, or a JSON object of touchpoints (repeat for up to 10)")
	f.StringVarP(&file, "file", "f", "", "read the JSON request from this file (- for stdin)")
	f.BoolVarP(&edit, "edit", "e", false, "fill the request in the built-in editor, starting from the flags given")
	return apiCommand(cmd)
}

func performanceTemplate(r *api.PerformanceRequest) string {
	variants := make([]any, 0, len(r.Variants))
	for _, v := range r.Variants {
		variants = append(variants, v)
	}
	if len(variants) == 0 {
		variants = append(variants, api.Touchpoints{"text": ""})
	}
	help := append([]string{
		fmt.Sprintf("Predict the performance of 1 to %d variants. Each variant maps touchpoint keys to copy.", api.MaxPerformanceVariants),
	}, accountHelp()...)
	return template(help, object{
		{"channel", orDefault(r.Channel, api.DefaultChannel)},
		{"displayChannel", r.DisplayChannel},
		{"account", accountTemplate(r.Account)},
		{"variants", variants},
	})
}

func renderPerformance(w io.Writer, r *api.PerformanceResponse) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "Model\t%s\nLanguage\t%s\nLocale source\t%s\n\n", dashIfEmpty(r.Model), dashIfEmpty(r.ResolvedLanguage), dashIfEmpty(r.LocaleSource))
	_, _ = fmt.Fprintln(tw, "#\tPerformance (%)\tModel\tText")
	for i, v := range r.Results {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", i, orDash(v.Performance, "%.2f"), dashIfEmpty(v.Model), oneLine(v.Text))
	}
	return tw.Flush()
}
