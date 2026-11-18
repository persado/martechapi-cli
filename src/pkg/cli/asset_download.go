package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

func newAssetDownloadCmd(g *globalFlags) *cobra.Command {
	var (
		out   string
		force bool
		edit  bool
	)
	cmd := &cobra.Command{
		Use:   "download ASSET_ID",
		Short: "Download an asset's file (GET /v1/asset/retrieve?stream=true)",
		Long: `Download an asset's file through the API.

Without --out the file is saved in the current directory under the asset's own
file name. An existing file is never overwritten unless you pass --force. --out
PATH saves to PATH instead; --out - writes the bytes to stdout. The file is
written to a temporary file first and renamed, so a failed download leaves no
partial file. The saved path and size are printed on stderr.`,
		Example: `  persadomapicli asset download 0b9a3a8e-5d1f-4c53-9a77-3f2d6c1e8b40
  persadomapicli asset download 0b9a3a8e-5d1f-4c53-9a77-3f2d6c1e8b40 --out brief.pdf --force
  persadomapicli asset download 0b9a3a8e-5d1f-4c53-9a77-3f2d6c1e8b40 --out - > brief.pdf`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := assetIDRequest{}
			if len(args) == 1 {
				req.ID = args[0]
			}
			if err := finalize(cmd, edit, "Download an asset (GET /v1/asset/retrieve)", &req, assetIDTemplate("Download an asset's file.")); err != nil {
				return err
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			if out != "" && out != "-" && !force {
				if err := refuseExisting(out); err != nil {
					return err
				}
			}
			resp, err := c.DownloadAsset(cmd.Context(), req.ID)
			if err != nil {
				var apiErr *api.Error
				if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
					return fmt.Errorf("asset %s not found or expired; list your assets with: persadomapicli asset list: %w", req.ID, err)
				}
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			if out == "-" {
				if _, err := io.Copy(cmd.OutOrStdout(), resp.Body); err != nil {
					return fmt.Errorf("write to stdout: %w", err)
				}
				return nil
			}
			if out == "" {
				out = downloadName(resp.Header.Get("X-Asset-Filename"), req.ID)
				if !force {
					if err := refuseExisting(out); err != nil {
						return err
					}
				}
			}
			n, err := saveAtomic(out, resp.Body)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Saved %s (%d bytes)\n", out, n)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVarP(&out, "out", "O", "", "save to this path, or - for stdout (default: the asset's file name in the current directory)")
	f.BoolVar(&force, "force", false, "overwrite an existing file")
	f.BoolVarP(&edit, "edit", "e", false, "fill the asset id in the built-in editor")
	return apiCommand(cmd)
}

// refuseExisting fails when p already exists.
func refuseExisting(p string) error {
	if _, err := os.Lstat(p); err == nil {
		return fmt.Errorf("%s already exists; use --force to overwrite it or --out to choose another path", p)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check %s: %w", p, err)
	}
	return nil
}

// downloadName is the base name of the percent-encoded X-Asset-Filename, or the id when it has none.
func downloadName(header, id string) string {
	name, err := url.PathUnescape(header)
	if err != nil {
		name = header
	}
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" || name == ".." || strings.ContainsRune(name, 0) {
		return id
	}
	return name
}

// saveAtomic writes r to a temporary file next to p, then renames it to p.
func saveAtomic(p string, r io.Reader) (int64, error) {
	tmp, err := os.CreateTemp(filepath.Dir(p), ".persadomapicli-*.part")
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	_ = tmp.Chmod(0o644)
	n, err := io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), p)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return 0, fmt.Errorf("save %s: %w", p, err)
	}
	return n, nil
}
