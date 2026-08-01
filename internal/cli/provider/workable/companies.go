package workable

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	provider "github.com/amikai/openings-mcp/internal/provider/workable"
)

func companiesCmd() *cli.Command {
	return &cli.Command{
		Name:      "companies",
		Usage:     "list every curated Workable company's account slug, one per line",
		UsageText: "openings-cli workable companies",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := rejectArgs(cmd); err != nil {
				return err
			}
			return runCompanies(cmd)
		},
	}
}

// runCompanies prints one account slug per line, sorted by company name —
// the form the other subcommands take as --company, so the output pipes
// straight into them. It makes no network call, and it is the one subcommand
// that does not emit JSON: the display names and the rest of
// internal/provider/workable/companies.yaml are not what a caller needs here.
func runCompanies(cmd *cli.Command) error {
	w := cmd.Root().Writer
	for _, c := range provider.Companies {
		if _, err := fmt.Fprintln(w, c.Account); err != nil {
			return err
		}
	}
	return nil
}
