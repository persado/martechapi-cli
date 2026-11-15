// Package cli defines the persadomapicli command tree.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "persadomapicli",
		Short:         "Persado Martech API command-line tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd())
	return root
}

// Run executes the command tree with args (program name excluded), writing to
// stdout and stderr, and returns the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}
