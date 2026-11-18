package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"
	"time"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

// assetIDRequest holds the id argument of "asset get" and "asset download".
type assetIDRequest struct {
	ID string `json:"id"`
}

func (r *assetIDRequest) Validate() error {
	if !api.ValidAssetID(r.ID) {
		return api.ValidationError{{Field: "id", Reason: fmt.Sprintf("%q is not an asset id (a UUID, as printed by asset create or asset list)", r.ID)}}
	}
	return nil
}

func assetIDTemplate(help string) func(*assetIDRequest) string {
	return func(r *assetIDRequest) string {
		return template([]string{help, "id: the asset UUID, as printed by asset create or asset list."}, object{{"id", r.ID}})
	}
}

func newAssetGetCmd(g *globalFlags) *cobra.Command {
	var edit bool
	cmd := &cobra.Command{
		Use:   "get ASSET_ID",
		Short: "Show an asset's metadata and a one-hour download link (GET /v1/asset/retrieve)",
		Long: `Show an asset's metadata and a presigned download URL that is valid for one hour.

Anyone who holds the URL can read the file until it expires, so do not share it
unless you mean to. To save the file, use "persadomapicli asset download".`,
		Example: `  persadomapicli asset get 0b9a3a8e-5d1f-4c53-9a77-3f2d6c1e8b40
  persadomapicli asset get 0b9a3a8e-5d1f-4c53-9a77-3f2d6c1e8b40 -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := assetIDRequest{}
			if len(args) == 1 {
				req.ID = args[0]
			}
			if err := finalize(cmd, edit, "Get an asset (GET /v1/asset/retrieve)", &req, assetIDTemplate("Show an asset's metadata and a one-hour download link.")); err != nil {
				return err
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			resp, err := c.AssetLink(cmd.Context(), req.ID)
			if err != nil {
				var apiErr *api.Error
				if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
					return fmt.Errorf("asset %s not found or expired; list your assets with: persadomapicli asset list: %w", req.ID, err)
				}
				return err
			}
			if g.output == outputJSON {
				err = printJSON(cmd.OutOrStdout(), []byte(resp.Raw))
			} else {
				err = renderAssetURL(cmd.OutOrStdout(), &resp.Value)
			}
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Note: anyone with this URL can read the file until it expires.")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "fill the asset id in the built-in editor")
	return apiCommand(cmd)
}

func renderAssetURL(w io.Writer, a *api.AssetURL) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "ID\t%s\nFilename\t%s\nType\t%s\nSize\t%d\nVersion\t%d\nExpires\t%s\nURL\t%s\nURL expires\t%s\n",
		a.ID, a.Filename, a.MimeType, a.Size, a.Version, a.ExpiresAt, a.URL, truncSeconds(a.URLExpiresAt))
	return tw.Flush()
}

// truncSeconds drops the fractional seconds of an RFC 3339 timestamp; other text is returned as is.
func truncSeconds(s string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.Truncate(time.Second).Format(time.RFC3339)
}
