package workable

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/urfave/cli/v3"

	provider "github.com/amikai/openings-mcp/internal/provider/workable"
)

func detailCmd() *cli.Command {
	return &cli.Command{
		Name:      "detail",
		Usage:     "print full description of a job",
		UsageText: "workable detail --company COMPANY --shortcode SHORTCODE",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "company",
				Usage: `Workable account subdomain from the careers URL, e.g. "blueground" in apply.workable.com/blueground`,
			},
			&cli.StringFlag{
				Name:     "shortcode",
				Usage:    "job shortcode from a search result",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := rejectArgs(cmd); err != nil {
				return err
			}
			return runDetail(ctx, cmd)
		},
	}
}

// runDetail fetches one job in full via the v2 detail endpoint, which — unlike
// search — 404s for an unknown shortcode.
func runDetail(ctx context.Context, cmd *cli.Command) error {
	account, err := normalizeCompany(cmd.String("company"))
	if err != nil {
		return err
	}
	shortcode := cmd.String("shortcode")

	ctx, cancel := context.WithTimeout(ctx, cmd.Duration("timeout"))
	defer cancel()

	client, err := newClient(cmd)
	if err != nil {
		return err
	}

	res, err := client.GetJob(ctx, provider.GetJobParams{Account: account, Shortcode: shortcode})
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *provider.JobDetail:
		return json.NewEncoder(cmd.Root().Writer).Encode(d)
	case *provider.NotFound:
		return fmt.Errorf("job %q not found for company %q", shortcode, account)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}
