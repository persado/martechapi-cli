package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/persado/martechapi-cli/src/pkg/api"
	"github.com/spf13/cobra"
)

// pollInterval is the pause between job reads while waiting. Tests shorten it.
var pollInterval = 2 * time.Second

// errJobFailed makes the command exit 1 after the failed job was printed.
var errJobFailed = errors.New("job failed")

// waitFlags are the flags of commands that start or read a job.
type waitFlags struct {
	async   bool
	timeout time.Duration
}

func (w *waitFlags) register(cmd *cobra.Command, startsJob bool) {
	if startsJob {
		cmd.Flags().BoolVar(&w.async, "async", false, "print the job as soon as it is accepted, without waiting for the result")
	}
	cmd.Flags().DurationVar(&w.timeout, "timeout", 15*time.Minute, "longest time to wait for the job to finish")
}

func newJobCmd(g *globalFlags) *cobra.Command {
	var (
		w    waitFlags
		edit bool
	)
	cmd := &cobra.Command{
		Use:   "job JOB_ID",
		Short: "Read a score or optimize job, waiting for it to finish",
		Long: `Read a score or optimize job by id (job_ followed by 32 hex digits).

By default the command waits until the job has completed or failed, then prints
the result. Use --no-wait to print the job as it is now. Jobs are kept for 24
hours after they were created.`,
		Example: `  persadomapicli job job_0123456789abcdef0123456789abcdef
  persadomapicli job job_0123456789abcdef0123456789abcdef --no-wait -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := jobRequest{}
			if len(args) == 1 {
				req.ID = args[0]
			}
			if err := finalize(cmd, edit, "Read a job", &req, jobTemplate); err != nil {
				return err
			}
			c, err := g.client()
			if err != nil {
				return err
			}
			resp, err := c.Job(cmd.Context(), req.ID)
			if err != nil {
				var ae *api.Error
				if errors.As(err, &ae) && ae.Status == http.StatusNotFound {
					return fmt.Errorf("job %s not found: jobs are kept for 24 hours after they were created: %w", req.ID, err)
				}
				return err
			}
			job := &resp.Value
			raw := []byte(resp.Raw)
			if !w.async && !job.Done() {
				if job, raw, err = waitJob(cmd.Context(), c, job, w.timeout, cmd.ErrOrStderr()); err != nil {
					return err
				}
			}
			return printJob(cmd.OutOrStdout(), g.output, job, raw)
		},
	}
	w.register(cmd, false)
	cmd.Flags().BoolVar(&w.async, "no-wait", false, "print the job as it is now, even while it is still running")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "fill the job id in the built-in editor")
	return apiCommand(cmd)
}

type jobRequest struct {
	ID string `json:"id"`
}

func (r *jobRequest) Validate() error {
	if !api.ValidJobID(r.ID) {
		return api.ValidationError{{Field: "id", Reason: fmt.Sprintf("%q is not a job id (job_ followed by 32 hex digits)", r.ID)}}
	}
	return nil
}

func jobTemplate(r *jobRequest) string {
	return template([]string{
		"Read a job. id: job_ followed by 32 hex digits, as printed by score or optimize.",
	}, object{{"id", r.ID}})
}

// waitJob reads job until it is done or timeout passes, reporting progress on progress.
func waitJob(ctx context.Context, c *api.Client, job *api.Job, timeout time.Duration, progress io.Writer) (*api.Job, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, _ = fmt.Fprintf(progress, "Waiting for %s %s", job.Kind, job.ID)
	defer func() { _, _ = fmt.Fprintln(progress) }()
	stillRunning := func() error {
		return fmt.Errorf("job %s still running after %s; read it later with: persadomapicli job %s", job.ID, timeout, job.ID)
	}
	var raw []byte
	for !job.Done() {
		t := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, nil, stillRunning()
			}
			return nil, nil, ctx.Err()
		case <-t.C:
		}
		resp, err := c.Job(ctx, job.ID)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, nil, stillRunning()
			}
			return nil, nil, err
		}
		job, raw = &resp.Value, resp.Raw
		_, _ = fmt.Fprint(progress, ".")
	}
	return job, raw, nil
}

// runJob starts a job with start (passing the server-side wait in seconds, 0 with --async),
// waits for it unless --async, and prints it.
func runJob(cmd *cobra.Command, g *globalFlags, w waitFlags, c *api.Client, start func(wait int) (*api.Response[api.Job], error)) error {
	wait := api.MaxWaitSeconds
	if w.async {
		wait = 0
	}
	resp, err := start(wait)
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
}

// printJob prints job as text or raw JSON. A failed job also returns errJobFailed.
func printJob(w io.Writer, format string, job *api.Job, raw []byte) error {
	var err error
	if format == outputJSON {
		err = printJSON(w, raw)
	} else {
		err = renderJob(w, job)
	}
	if err != nil {
		return err
	}
	if job.Status == api.JobFailed {
		return errJobFailed
	}
	return nil
}

func renderJob(w io.Writer, job *api.Job) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "Job\t%s\nKind\t%s\nStatus\t%s\nCreated\t%s\nExpires\t%s\n", job.ID, job.Kind, job.Status, job.CreatedAt, job.ExpiresAt)
	if !job.Done() {
		_, _ = fmt.Fprintf(tw, "Next\tpersadomapicli job %s\n", job.ID)
	}
	if e := job.Error; e != nil {
		_, _ = fmt.Fprintf(tw, "Error\t%s: %s\n", e.Code, e.Message)
		if len(e.Categories) > 0 {
			_, _ = fmt.Fprintf(tw, "Personal data\t%s\n", strings.Join(e.Categories, ", "))
		}
	}
	r := job.Result
	if r == nil {
		return tw.Flush()
	}
	_, _ = fmt.Fprintf(tw, "Brand\t%s\nChannel\t%s\n", r.Brand, r.Channel)
	if !r.Complete {
		_, _ = fmt.Fprintln(tw, "Result\tpartial: the job is still running and values can change")
	}
	if !r.ComplianceApplicable {
		_, _ = fmt.Fprintln(tw, "Compliance\tnot applicable to this request")
	}
	if r.Notice != nil {
		_, _ = fmt.Fprintf(tw, "Notice\t%s: %s\n", r.Notice.Code, r.Notice.Message)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	switch {
	case len(r.Variants) > 0:
		_, _ = fmt.Fprintln(w, "\nVariants")
		return variantTable(w, "#", r.Variants, func(v api.Variant) *int { return v.Index })
	case r.Control != nil || len(r.Alternatives) > 0:
		if r.Control != nil {
			_, _ = fmt.Fprintln(w, "\nOriginal")
			if err := variantTable(w, "", []api.Variant{*r.Control}, nil); err != nil {
				return err
			}
		}
		alts := slices.Clone(r.Alternatives)
		slices.SortStableFunc(alts, func(a, b api.Variant) int { return deref(a.Rank) - deref(b.Rank) })
		_, _ = fmt.Fprintln(w, "\nAlternatives (best first)")
		return variantTable(w, "Rank", alts, func(v api.Variant) *int { return v.Rank })
	}
	return nil
}

func variantTable(w io.Writer, numHeader string, vs []api.Variant, num func(api.Variant) *int) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	// A nil num omits the number column.
	lead, leadHeader := func(api.Variant) string { return "" }, ""
	if num != nil {
		lead = func(v api.Variant) string { return orDash(num(v), "%d") + "\t" }
		leadHeader = numHeader + "\t"
	}
	_, _ = fmt.Fprintf(tw, "%sPerformance\tBand\tBrand fit\tCompliance\tText\n", leadHeader)
	for _, v := range vs {
		compliance := "-"
		if v.Compliance != nil {
			compliance = v.Compliance.Risk
			if compliance == "" {
				compliance = v.Compliance.State
			}
		}
		_, _ = fmt.Fprintf(tw, "%s%s\t%s\t%s\t%s\t%s\n", lead(v), orDash(v.Performance, "%d"),
			orDash(v.PerformanceBand, "%s"), orDash(v.BrandAlignment, "%d/5"), compliance, oneLine(v.Text))
	}
	return tw.Flush()
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
