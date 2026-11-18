package cli

import "github.com/spf13/cobra"

func newAssetCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asset",
		Short: "Upload, list and download assets (files kept for later API runs)",
		Long: `Upload, list and download assets: files stored for your API key's owner, such as
briefs or images, that later API runs can use. Each asset expires after 1 to 90
days (30 by default).`,
	}
	cmd.AddCommand(newAssetCreateCmd(g), newAssetListCmd(g), newAssetGetCmd(g), newAssetDownloadCmd(g))
	return cmd
}
