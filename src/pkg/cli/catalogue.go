package cli

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

// catalogueSections are the values of --section, in print order.
var catalogueSections = []string{"channels", "display-channels", "industries", "limits"}

func newCatalogueCmd(g *globalFlags) *cobra.Command {
	var section string
	cmd := &cobra.Command{
		Use:   "catalogue",
		Short: "List channels, display channels, industries and limits (GET /v1/catalogue)",
		Long: `Read the catalogue: the channels with their touchpoint keys, the default channel,
the display channels, the industries and the server's limits. Labels follow --lang.

Use --section to print one part of the text output. With -o json the whole
document is printed as the server sent it.`,
		Example: `  persadomapicli catalogue
  persadomapicli catalogue --section channels
  persadomapicli catalogue --lang fr --section industries
  persadomapicli catalogue -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if section != "" && !slices.Contains(catalogueSections, section) {
				return fmt.Errorf("--section must be one of %s, not %q", strings.Join(catalogueSections, ", "), section)
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			resp, err := c.Catalogue(cmd.Context())
			if err != nil {
				return err
			}
			if g.output == outputJSON {
				return printJSON(cmd.OutOrStdout(), resp.Raw)
			}
			return renderCatalogue(cmd.OutOrStdout(), &resp.Value, section)
		},
	}
	cmd.Flags().StringVar(&section, "section", "", "print one section: "+strings.Join(catalogueSections, ", ")+" (text output)")
	return apiCommand(cmd)
}

// renderCatalogue prints every section, or only section when it is not empty.
func renderCatalogue(w io.Writer, cat *api.Catalogue, section string) error {
	sections := catalogueSections
	if section != "" {
		sections = []string{section}
	}
	for i, s := range sections {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}
		if err := renderCatalogueSection(w, cat, s); err != nil {
			return err
		}
	}
	return nil
}

func renderCatalogueSection(w io.Writer, cat *api.Catalogue, section string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	switch section {
	case "channels":
		_, _ = fmt.Fprintf(w, "Channels (default %s)\n", orDefault(cat.DefaultChannel, "-"))
		_, _ = fmt.Fprintln(tw, "Key\tLabel\tTouchpoints")
		for _, c := range cat.Channels {
			tps := make([]string, 0, len(c.Touchpoints))
			for _, t := range c.Touchpoints {
				tps = append(tps, touchpointText(t))
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", orDefault(c.Key, "-"), orDefault(c.Label, "-"), orDefault(strings.Join(tps, ", "), "-"))
		}
	case "display-channels":
		_, _ = fmt.Fprintln(w, "Display channels")
		writeKeyLabels(tw, cat.DisplayChannels)
	case "industries":
		_, _ = fmt.Fprintln(w, "Industries")
		writeKeyLabels(tw, cat.Industries)
	case "limits":
		_, _ = fmt.Fprintln(w, "Limits")
		_, _ = fmt.Fprintln(tw, "Limit\tValue")
		l := cat.Limits
		_, _ = fmt.Fprintf(tw, "Max body bytes\t%s\n", dashIfZero(l.MaxBodyBytes))
		_, _ = fmt.Fprintf(tw, "Max touchpoint length\t%s\n", dashIfZero(l.MaxTouchpointLength))
		_, _ = fmt.Fprintf(tw, "Max score variants\t%s\n", dashIfZero(l.MaxScoreVariants))
		_, _ = fmt.Fprintf(tw, "Max wait (seconds)\t%s\n", dashIfZero(l.MaxWaitSeconds))
	}
	return tw.Flush()
}

func writeKeyLabels(tw *tabwriter.Writer, items []api.KeyLabel) {
	_, _ = fmt.Fprintln(tw, "Key\tLabel")
	for _, it := range items {
		_, _ = fmt.Fprintf(tw, "%s\t%s\n", orDefault(it.Key, "-"), orDefault(it.Label, "-"))
	}
}

// touchpointText formats a touchpoint as "key (label)", or just the key without a label.
func touchpointText(t api.KeyLabel) string {
	if t.Label == "" {
		return t.Key
	}
	return t.Key + " (" + t.Label + ")"
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func dashIfZero(n int) string {
	if n == 0 {
		return "-"
	}
	return strconv.Itoa(n)
}
