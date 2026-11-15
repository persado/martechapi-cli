// Command martechapi is the martechapi-cli entry point.
package main

import (
	"context"
	"os"

	"github.com/persado/martechapi-cli/src/pkg/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
