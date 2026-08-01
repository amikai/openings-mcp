// Command openings-cli is the debug CLI for the job-listing providers,
// one subcommand per provider.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	workable "github.com/amikai/openings-mcp/internal/cli/provider/workable"
)

func main() {
	root := &cli.Command{
		Name:      "openings-cli",
		Usage:     "inspect a provider's public job board API",
		UsageText: "openings-cli <provider> [FLAGS] <command> [FLAGS]",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cli.ShowAppHelp(cmd)
			return errors.New("a provider subcommand is required")
		},
		Commands: []*cli.Command{
			workable.NewCmd(),
		},
	}

	if err := root.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}
