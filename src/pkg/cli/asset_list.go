package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

// assetListRequest holds the inputs of "asset list".
type assetListRequest struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
	All    bool   `json:"all"`
}

func (r *assetListRequest) Validate() error {
	var errs api.ValidationError
	if r.Limit != 0 && (r.Limit < 1 || r.Limit > api.MaxAssetListLimit) {
		errs = append(errs, api.ErrorDetail{Field: "limit", Reason: fmt.Sprintf("must be 1 to %d (0 uses the server default), got %d", api.MaxAssetListLimit, r.Limit)})
	}
	if len(r.Cursor) > 512 {
		errs = append(errs, api.ErrorDetail{Field: "cursor", Reason: "must be at most 512 bytes"})
	}
	if r.All && r.Cursor != "" {
		errs = append(errs, api.ErrorDetail{Field: "cursor", Reason: "cannot be combined with all; all starts from the first page"})
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func assetListTemplate(r *assetListRequest) string {
	return template([]string{
		fmt.Sprintf("List assets, newest first. limit: 1 to %d per page, 0 for the server default (50).", api.MaxAssetListLimit),
		"cursor: the opaque value printed by a previous page; empty for the first page.",
		"all: true follows every page; then cursor must stay empty.",
	}, object{{"limit", r.Limit}, {"cursor", r.Cursor}, {"all", r.All}})
}

func newAssetListCmd(g *globalFlags) *cobra.Command {
	var (
		req  assetListRequest
		edit bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your assets, newest first (GET /v1/asset/list)",
		Long: fmt.Sprintf(`List the assets stored for your API key's owner, newest first. Expired assets are
not listed.

A page holds up to --limit assets (1 to %d, default 50) and can hold fewer, even
none, while more pages follow. When more pages exist the next cursor is printed
on stderr; pass it back with --cursor. --all follows every page. With -o json
the page is printed as the server sent it; with --all, as {"assets":[...]}.`, api.MaxAssetListLimit),
		Example: `  persadomapicli asset list
  persadomapicli asset list --limit 200 --cursor CURSOR
  persadomapicli asset list --all -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := finalize(cmd, edit, "List assets (GET /v1/asset/list)", &req, assetListTemplate); err != nil {
				return err
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			var (
				assets []api.Asset
				raws   []json.RawMessage
				cursor = req.Cursor
				last   []byte
			)
			for {
				resp, err := c.ListAssets(cmd.Context(), req.Limit, cursor)
				if err != nil {
					return err
				}
				assets = append(assets, resp.Value.Assets...)
				last, cursor = []byte(resp.Raw), resp.Value.NextCursor
				if req.All {
					var page struct {
						Assets []json.RawMessage `json:"assets"`
					}
					if err := json.Unmarshal(resp.Raw, &page); err != nil {
						return fmt.Errorf("read asset list: %w", err)
					}
					raws = append(raws, page.Assets...)
				}
				if !req.All || cursor == "" {
					break
				}
			}
			if g.output == outputJSON {
				if req.All {
					if raws == nil {
						raws = []json.RawMessage{}
					}
					last, err = json.Marshal(map[string]any{"assets": raws})
					if err != nil {
						return err
					}
				}
				err = printJSON(cmd.OutOrStdout(), last)
			} else {
				err = renderAssets(cmd.OutOrStdout(), assets)
			}
			if err != nil {
				return err
			}
			if cursor != "" {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "More assets exist. Next page: --cursor %s\n", cursor)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVar(&req.Limit, "limit", 0, fmt.Sprintf("assets per page, 1 to %d (default 50)", api.MaxAssetListLimit))
	f.StringVar(&req.Cursor, "cursor", "", "continue from this cursor, as printed by a previous page")
	f.BoolVar(&req.All, "all", false, "follow every page and list all assets")
	f.BoolVarP(&edit, "edit", "e", false, "fill the options in the built-in editor, starting from the flags given")
	return apiCommand(cmd)
}

func renderAssets(w io.Writer, assets []api.Asset) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tFilename\tType\tSize\tCreated\tExpires\tAttributes")
	for _, a := range assets {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n", a.ID, a.Filename, a.MimeType, a.Size, shortTime(a.CreatedAt), shortTime(a.ExpiresAt), attrString(a.Attributes))
	}
	return tw.Flush()
}

func attrString(m map[string]string) string {
	if len(m) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + m[k]
	}
	return strings.Join(parts, ",")
}

// shortTime renders an RFC 3339 timestamp in UTC without fractions; other values pass through.
func shortTime(s string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.UTC().Format(time.RFC3339)
}
