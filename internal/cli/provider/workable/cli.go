package workable

import (
	"context"
	"errors"
	"time"

	"github.com/urfave/cli/v3"

	provider "github.com/amikai/openings-mcp/internal/provider/workable"
)

// NewCmd builds the "workable" root command and its four subcommands
// companies/search/detail/filters. --timeout and --base-url sit here because
// every subcommand that talks to the API needs them; --company is declared
// per subcommand instead, so "companies" does not advertise a flag it ignores.
func NewCmd() *cli.Command {
	return &cli.Command{
		Name:      "workable",
		Usage:     "debug CLI for the Workable job board API",
		UsageText: "workable [FLAGS] <companies|search|detail|filters> [FLAGS]",
		Flags: []cli.Flag{
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 60 * time.Second,
				Usage: "request timeout",
			},
			&cli.StringFlag{
				Name:   "base-url",
				Value:  provider.DefaultBaseURL,
				Hidden: true,
				Usage:  "Workable API origin (for tests)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cli.ShowAppHelp(cmd)
			return errors.New("a subcommand (companies, search, detail, or filters) is required")
		},
		Commands: []*cli.Command{
			companiesCmd(),
			searchCmd(),
			detailCmd(),
			filtersCmd(),
		},
	}
}
