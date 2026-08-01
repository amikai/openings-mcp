package workable

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/urfave/cli/v3"

	provider "github.com/amikai/openings-mcp/internal/provider/workable"
)

func filtersCmd() *cli.Command {
	return &cli.Command{
		Name:      "filters",
		Usage:     "list a company's search facets",
		UsageText: "workable filters --company COMPANY",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "company",
				Usage: `Workable account subdomain from the careers URL, e.g. "blueground" in apply.workable.com/blueground`,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := rejectArgs(cmd); err != nil {
				return err
			}
			return runFilters(ctx, cmd)
		},
	}
}

// runFilters dumps the account's facets — most usefully the numeric
// department ids that search's --department-id flag requires.
func runFilters(ctx context.Context, cmd *cli.Command) error {
	account, err := normalizeCompany(cmd.String("company"))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, cmd.Duration("timeout"))
	defer cancel()

	client, err := newClient(cmd)
	if err != nil {
		return err
	}

	res, err := client.ListJobFilters(ctx, provider.ListJobFiltersParams{Account: account})
	if err != nil {
		return err
	}

	facets, ok := res.(*provider.FiltersResponse)
	if !ok {
		return fmt.Errorf("company %q not found on Workable (account removed?)", account)
	}

	return json.NewEncoder(cmd.Root().Writer).Encode(facets)
}
