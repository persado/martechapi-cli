package cli

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

// assetMaxBytes is the server's default file size limit.
const assetMaxBytes = 10 << 20

// assetEdit is the shared metadata of an upload, as edited with --edit.
// Filename is used only when one file is uploaded; an empty MimeType means
// detect it per file.
type assetEdit struct {
	Filename      string            `json:"filename,omitempty"`
	MimeType      string            `json:"mimeType,omitempty"`
	ExpiresInDays int               `json:"expiresInDays,omitempty"`
	Owner         string            `json:"owner,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// Validate checks the edited metadata; the file-specific parts are filled in
// with placeholders when empty, because they come from the file.
func (e *assetEdit) Validate() error {
	m := e.metadata()
	if m.Filename == "" {
		m.Filename = "file"
	}
	if m.MimeType == "" {
		m.MimeType = "application/octet-stream"
	}
	m.Length = 1
	return m.Validate(0)
}

func (e *assetEdit) metadata() api.AssetMetadata {
	return api.AssetMetadata{Filename: e.Filename, MimeType: e.MimeType, ExpiresInDays: e.ExpiresInDays,
		Owner: e.Owner, Attributes: e.Attributes}
}

type assetFile struct {
	path string
	meta api.AssetMetadata
}

func newAssetCreateCmd(g *globalFlags) *cobra.Command {
	var (
		name     string
		mimeType string
		days     int
		owner    string
		attrs    []string
		edit     bool
	)
	cmd := &cobra.Command{
		Use:     "create FILE...",
		Aliases: []string{"upload"},
		Short:   "Upload one or more files as assets (POST /v1/asset/create)",
		Long: fmt.Sprintf(`Upload files as assets, one request per file, in the order given.

Every file is checked locally before the first upload: it must be a non-empty
regular file of at most %d MiB, with valid metadata. The file name stored is the path as
given, with / separators; an absolute path or one containing .. is stored as its base
name only (--name replaces it, for a single file only). The media type comes from the
extension, else from the content; --mime-type overrides it. Assets expire after
--expires-in-days (1 to %d, default 30). --owner must be your key owner's e-mail address.

A failed upload does not stop the others; failures are listed on stderr and the
exit code is 1 if any upload failed. With -o json the created assets are printed
as one JSON array.`, assetMaxBytes>>20, api.MaxExpiresInDays),
		Example: `  persadomapicli asset create brief.pdf
  persadomapicli asset create docs/*.pdf --expires-in-days 7 --attr campaign=spring
  persadomapicli asset upload logo.png --name brand/logo.png -o json
  persadomapicli asset create brief.pdf --edit`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name != "" && len(args) > 1 {
				return errors.New("--name is valid with one file only")
			}
			shared := assetEdit{Filename: name, MimeType: mimeType, ExpiresInDays: days, Owner: owner}
			if len(attrs) > 0 {
				shared.Attributes = map[string]string{}
				for _, a := range attrs {
					k, v, ok := strings.Cut(a, "=")
					if !ok || k == "" {
						return fmt.Errorf("--attr %q must be KEY=VALUE", a)
					}
					shared.Attributes[k] = v
				}
			}
			files, err := prepareAssets(args)
			if err != nil {
				return err
			}
			if len(files) == 1 {
				if shared.Filename == "" {
					shared.Filename = files[0].meta.Filename
				}
				if shared.MimeType == "" {
					shared.MimeType = files[0].meta.MimeType
				}
			}
			if err := finalize(cmd, edit, "Asset upload (POST /v1/asset/create)", &shared,
				func(e *assetEdit) string { return assetTemplate(e, len(files) == 1) }); err != nil {
				return err
			}
			if len(files) > 1 && shared.Filename != "" {
				return errors.New("filename is valid with one file only")
			}
			var bad []string
			for i := range files {
				m := &files[i].meta
				m.Filename = orDefault(shared.Filename, m.Filename)
				m.MimeType = orDefault(shared.MimeType, m.MimeType)
				m.ExpiresInDays, m.Owner, m.Attributes = shared.ExpiresInDays, shared.Owner, shared.Attributes
				if err := m.Validate(assetMaxBytes); err != nil {
					bad = append(bad, fmt.Sprintf("%s:\n%v", files[i].path, err))
				}
			}
			if len(bad) > 0 {
				return fmt.Errorf("invalid upload (nothing was sent):\n%s", strings.Join(bad, "\n"))
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			return uploadAssets(cmd, g, c, files)
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "file name to store instead of the path (one file only)")
	f.StringVar(&mimeType, "mime-type", "", "media type, such as application/pdf (default: from the extension or content)")
	f.IntVar(&days, "expires-in-days", 0, "days until the asset expires, 1 to 90 (default 30)")
	f.StringVar(&owner, "owner", "", "owner e-mail address; must match the API key's owner")
	f.StringArrayVar(&attrs, "attr", nil, "attribute KEY=VALUE (repeatable, up to 20)")
	f.BoolVarP(&edit, "edit", "e", false, "edit the shared metadata in the built-in editor, starting from the flags given")
	return apiCommand(cmd)
}

// prepareAssets checks every path and derives its metadata.
func prepareAssets(paths []string) ([]assetFile, error) {
	var (
		files []assetFile
		bad   []string
	)
	for _, p := range paths {
		st, err := os.Stat(p)
		switch {
		case err != nil:
			bad = append(bad, fmt.Sprintf("%s: %v", p, errors.Unwrap(err)))
			continue
		case st.IsDir():
			bad = append(bad, p+": is a directory; give files (for example "+p+"/*)")
			continue
		case !st.Mode().IsRegular():
			bad = append(bad, p+": is not a regular file")
			continue
		case st.Size() == 0:
			bad = append(bad, p+": is empty; empty files cannot be uploaded")
			continue
		}
		mt, err := detectMime(p)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: %v", p, errors.Unwrap(err)))
			continue
		}
		files = append(files, assetFile{path: p, meta: api.AssetMetadata{Filename: storedName(p), MimeType: mt, Length: st.Size()}})
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("cannot upload (nothing was sent):\n%s", strings.Join(bad, "\n"))
	}
	return files, nil
}

// storedName is the name sent to the server: slash-separated, and only the
// base name for an absolute path or one that climbs out with "..", so local
// directory names are not disclosed.
func storedName(p string) string {
	s := filepath.ToSlash(p)
	if filepath.IsAbs(p) || strings.HasPrefix(s, "/") {
		return filepath.Base(p)
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return filepath.Base(p)
		}
	}
	return s
}

// detectMime uses the extension, else the first 512 bytes of the file.
func detectMime(path string) (string, error) {
	if t := mime.TypeByExtension(filepath.Ext(path)); t != "" {
		return t, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}

// uploadAssets sends the files one by one and reports each as it completes.
func uploadAssets(cmd *cobra.Command, g *globalFlags, c *api.Client, files []assetFile) error {
	out, errw := cmd.OutOrStdout(), cmd.ErrOrStderr()
	text := g.output != outputJSON
	if text {
		_, _ = fmt.Fprintf(out, "%-36s  %10s  %-20s  %s\n", "ID", "Size", "Expires", "Filename")
	}
	var (
		created []string
		failed  int
	)
	for _, af := range files {
		resp, err := uploadOne(cmd, c, af)
		if err != nil {
			if cmd.Context().Err() != nil {
				return err
			}
			failed++
			_, _ = fmt.Fprintf(errw, "%s: %v\n", af.path, err)
			continue
		}
		created = append(created, string(resp.Raw))
		if text {
			a := resp.Value
			_, _ = fmt.Fprintf(out, "%-36s  %10d  %-20s  %s\n", a.ID, a.Size, a.ExpiresAt, a.Filename)
		}
	}
	if !text {
		if err := printJSON(out, []byte("["+strings.Join(created, ",")+"]")); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d uploads failed", failed, len(files))
	}
	return nil
}

func uploadOne(cmd *cobra.Command, c *api.Client, af assetFile) (*api.Response[api.Asset], error) {
	f, err := os.Open(af.path)
	if err != nil {
		return nil, errors.Unwrap(err)
	}
	defer func() { _ = f.Close() }()
	return c.CreateAsset(cmd.Context(), &af.meta, f)
}

func assetTemplate(e *assetEdit, single bool) string {
	help := []string{
		fmt.Sprintf("Metadata shared by every file uploaded. Each file must be 1 byte to %d MiB.", assetMaxBytes>>20),
		fmt.Sprintf("expiresInDays: 1 to %d (default 30)", api.MaxExpiresInDays),
		"owner (optional): e-mail address of the API key's owner",
		fmt.Sprintf("attributes (optional): up to %d KEY: VALUE pairs; key 1 to 128 of A-Za-z0-9._-, value up to 256 characters", api.MaxAssetAttributes),
	}
	var body object
	if single {
		help = append(help, "filename: 1 to 255 characters, as stored")
		body = object{{"filename", e.Filename}}
	}
	help = append(help, "mimeType: such as application/pdf; empty detects it from each file")
	body = append(body, field{"mimeType", e.MimeType})
	attrs := e.Attributes
	if attrs == nil {
		attrs = map[string]string{}
	}
	body = append(body, object{{"expiresInDays", e.ExpiresInDays}, {"owner", e.Owner}, {"attributes", attrs}}...)
	return template(help, body)
}
