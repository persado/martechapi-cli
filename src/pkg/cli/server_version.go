package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

func newServerVersionCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server-version",
		Short: "Print the API server's version, commit, build time, spec date and environment (GET /version)",
		Long: `Read the build of the Martech API server that --base-url points to: its version,
commit, build time, the date of the OpenAPI spec it serves and its environment name.

No API key is needed.`,
		Example: `  persadomapicli server-version
  persadomapicli server-version --base-url https://eapi.persado.com/api
  persadomapicli server-version -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := g.client()
			if err != nil {
				return err
			}
			resp, err := c.Version(cmd.Context())
			if err != nil {
				return err
			}
			if g.output == outputJSON {
				return printJSON(cmd.OutOrStdout(), resp.Raw)
			}
			return renderServerVersion(cmd.OutOrStdout(), &resp.Value)
		},
	}
	return apiCommand(cmd)
}

func renderServerVersion(w io.Writer, v *api.VersionInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "Version\t%s\nCommit\t%s\nBuild time\t%s\nSpec date\t%s\nEnvironment\t%s\n",
		dashIfEmpty(v.Version), dashIfEmpty(v.Commit), dashIfEmpty(v.BuildTime), dashIfEmpty(v.SpecDate), dashIfEmpty(v.Environment))
	return tw.Flush()
}
