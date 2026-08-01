// Command workable is a debug CLI for the Workable job board API.
package main

import (
	"context"
	"fmt"
	"os"

	workablecli "github.com/amikai/openings-mcp/internal/cli/provider/workable"
)

func main() {
	if err := workablecli.NewCmd().Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}
