package cli

import (
	"fmt"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

func newScoreCmd(g *globalFlags) *cobra.Command {
	var (
		acc      accountFlags
		w        waitFlags
		variants []string
		file     string
		edit     bool
	)
	cmd := &cobra.Command{
		Use:   "score",
		Short: "Score 1 to 4 variants of copy (POST /v1/score)",
		Long: fmt.Sprintf(`Score 1 to %d variants of marketing copy for predicted performance, brand fit
and compliance risk.

Give each variant with --variant: plain text for the channel's text touchpoint,
or a JSON object of touchpoints for multi-part channels, such as
'{"headline":"...","body":"..."}'. Start from a JSON request with --file, or
fill the request in the built-in editor with --edit. Flags override the file.

The run is a job. By default the command waits for it and prints the scores;
--async prints the job id at once (read it later with "persadomapicli job").`, api.MaxScoreVariants),
		Example: `  persadomapicli score -a "Acme Bank" --variant "Your card is waiting" --variant "Apply in minutes"
  persadomapicli score -a "Acme Bank" -c PUSH_NOTIFICATION --variant '{"headline":"Hi","body":"Apply today"}'
  persadomapicli score --file request.json -o json
  persadomapicli score --edit`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := &api.ScoreRequest{}
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
			if err := finalize(cmd, edit, "Score request (POST /v1/score)", req, scoreTemplate); err != nil {
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
			resp, err := c.Score(cmd.Context(), req, wait)
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
	f.StringArrayVarP(&variants, "variant", "v", nil, "a variant: plain text, or a JSON object of touchpoints (repeat for up to 4)")
	f.StringVarP(&file, "file", "f", "", "read the JSON request from this file (- for stdin)")
	f.BoolVarP(&edit, "edit", "e", false, "fill the request in the built-in editor, starting from the flags given")
	return apiCommand(cmd)
}

func scoreTemplate(r *api.ScoreRequest) string {
	variants := make([]any, 0, len(r.Variants))
	for _, v := range r.Variants {
		variants = append(variants, v)
	}
	if len(variants) == 0 {
		variants = append(variants, api.Touchpoints{"text": ""})
	}
	help := append([]string{
		fmt.Sprintf("Score 1 to %d variants. Each variant maps touchpoint keys to copy.", api.MaxScoreVariants),
	}, accountHelp()...)
	return template(help, object{
		{"channel", orDefault(r.Channel, api.DefaultChannel)},
		{"displayChannel", r.DisplayChannel},
		{"account", accountTemplate(r.Account)},
		{"variants", variants},
	})
}
