package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

func newUpliftCmd(g *globalFlags) *cobra.Command {
	var (
		acc        accountFlags
		variant    string
		categories []string
		file       string
		edit       bool
	)
	cmd := &cobra.Command{
		Use:   "uplift",
		Short: "Project the optimization potential and uplift of one variant (POST /v1/uplift)",
		Long: `Project how much better optimized copy could perform than one variant.

Reports the variant's performance, the potential an optimized variant can
reach, and per response category the uplift in percent: against the industry
today, against the industry once optimized, and total (optimized vs variant).
The call is synchronous and costs one run unit.

Give the variant with --variant: plain text for the channel's text touchpoint,
or a JSON object of touchpoints for multi-part channels. Narrow the categories
with --category (Click, Conversion, Open; repeatable; default Click and
Conversion). Start from a JSON request with --file, or fill the request in the
built-in editor with --edit. Flags override the file.`,
		Example: `  persadomapicli uplift -a "Acme Bank" --variant "Your card is waiting"
  persadomapicli uplift -a "Acme Bank" -c SMS_EMS --variant "Apply today" --category click --category open
  persadomapicli uplift --file request.json -o json
  persadomapicli uplift --edit`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := &api.UpliftRequest{}
			if file != "" {
				if err := loadFile(cmd, file, req); err != nil {
					return err
				}
			}
			acc.apply(cmd, &req.Channel, &req.DisplayChannel, &req.Account)
			if cmd.Flags().Changed("variant") {
				t, err := parseVariant(variant)
				if err != nil {
					return err
				}
				req.Variant = t
			}
			if cmd.Flags().Changed("category") {
				req.ResponseCategories = nil
				for _, c := range categories {
					req.ResponseCategories = append(req.ResponseCategories, normalizeCategory(c))
				}
			}
			if err := finalize(cmd, edit, "Uplift request (POST /v1/uplift)", req, upliftTemplate); err != nil {
				return err
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			resp, err := c.Uplift(cmd.Context(), req)
			if err != nil {
				return err
			}
			if g.output == outputJSON {
				return printJSON(cmd.OutOrStdout(), []byte(resp.Raw))
			}
			return renderUplift(cmd.OutOrStdout(), &resp.Value)
		},
	}
	acc.register(cmd)
	f := cmd.Flags()
	f.StringVarP(&variant, "variant", "v", "", "the variant: plain text, or a JSON object of touchpoints")
	f.StringArrayVar(&categories, "category", nil, "response category: "+strings.Join(api.ResponseCategories, ", ")+" (repeatable; default Click and Conversion)")
	f.StringVarP(&file, "file", "f", "", "read the JSON request from this file (- for stdin)")
	f.BoolVarP(&edit, "edit", "e", false, "fill the request in the built-in editor, starting from the flags given")
	return apiCommand(cmd)
}

// normalizeCategory maps any casing of a known category to its API casing.
func normalizeCategory(s string) string {
	for _, c := range api.ResponseCategories {
		if strings.EqualFold(s, c) {
			return c
		}
	}
	return s
}

func upliftTemplate(r *api.UpliftRequest) string {
	variant := r.Variant
	if len(variant) == 0 {
		variant = api.Touchpoints{"text": ""}
	}
	cats := r.ResponseCategories
	if cats == nil {
		cats = []string{}
	}
	help := append([]string{
		"Project the uplift of one variant. The variant maps touchpoint keys to copy.",
		"responseCategories: any of " + strings.Join(api.ResponseCategories, ", ") + "; leave empty for Click and Conversion",
	}, accountHelp()...)
	return template(help, object{
		{"channel", orDefault(r.Channel, api.DefaultChannel)},
		{"displayChannel", r.DisplayChannel},
		{"account", accountTemplate(r.Account)},
		{"variant", variant},
		{"responseCategories", cats},
	})
}

func renderUplift(w io.Writer, u *api.UpliftResponse) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "Performance\t%s\nPotential\t%s\nAt potential\t%t\nResolution\t%s\nChannel category\t%s\nVertical\t%s\n",
		orDash(u.Performance, "%.2f"), orDash(u.Potential, "%.2f"), u.AtPotential,
		orDash(u.Resolution, "%s"), orDash(u.ChannelCategory, "%s"), orDash(u.Vertical, "%s"))
	if n := u.Notice; n != nil {
		_, _ = fmt.Fprintf(tw, "Notice\t%s: %s\n", n.Code, n.Message)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(u.Uplift) == 0 {
		return nil
	}
	_, _ = fmt.Fprintln(w)
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "Category\tvs industry today\tvs industry projected\ttotal")
	for _, it := range u.Uplift {
		total := pct(it.Total)
		if it.Unavailable != nil {
			total += " (" + *it.Unavailable + ")"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", it.Category, pct(it.VsIndustryToday), pct(it.VsIndustryProjected), total)
	}
	return tw.Flush()
}

func pct(p *float64) string { return orDash(p, "%+.1f%%") }
