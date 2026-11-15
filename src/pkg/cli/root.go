// Package cli defines the persadomapicli command tree.
package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/persado/martechapi-cli/src/pkg/update"
	"github.com/persado/martechapi-cli/src/pkg/version"
	"github.com/spf13/cobra"
)

// annotationAPI marks a command that calls the Martech API. The root prints
// version.Banner on stderr before such a command runs.
const annotationAPI = "persado/api"

// updateCheck returns the latest released version. Tests replace it so that
// they do not reach the network.
var updateCheck = func(ctx context.Context) (string, error) {
	return update.Latest(ctx, http.DefaultClient, update.LatestURL)
}

// updateWait is the longest Run waits at exit for an update check still in flight.
var updateWait = 2 * time.Second

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "persadomapicli",
		Short:         "Persado Martech API command-line tool",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if _, ok := cmd.Annotations[annotationAPI]; !ok {
				return nil
			}
			_, err := fmt.Fprintln(cmd.ErrOrStderr(), version.Banner())
			return err
		},
	}
	root.AddCommand(newVersionCmd())
	return root
}

// Run executes the command tree with args (program name excluded), writing to
// stdout and stderr, and returns the process exit code. An update check runs
// alongside the command; if it reports a newer release within updateWait of
// the command finishing, Run prints an upgrade notice on stderr.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	latest := make(chan string, 1)
	check := updateCheck
	go func() {
		v, err := check(ctx)
		if err != nil {
			v = ""
		}
		latest <- v
	}()

	code := 0
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(stderr, "error:", err)
		code = 1
	}

	select {
	case v := <-latest:
		if update.Newer(version.Version, v) {
			_, _ = fmt.Fprintf(stderr, "\nNew version found: %s (installed: %s). To upgrade, run:\n  %s\n",
				v, version.Version, update.InstallCommand(runtime.GOOS))
		}
	case <-time.After(updateWait):
	}
	return code
}
