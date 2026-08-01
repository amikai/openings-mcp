package workable

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/urfave/cli/v3"

	provider "github.com/amikai/openings-mcp/internal/provider/workable"
)

func searchCmd() *cli.Command {
	return &cli.Command{
		Name:      "search",
		Usage:     "search jobs for a company",
		UsageText: "openings-cli workable search --company COMPANY [--keyword TEXT] [--country C] [--region R] [--city CITY] [--department-id ID] [--workplace W] [--worktype W] [--remote] [--page-token CURSOR]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "company",
				Usage: `Workable account subdomain from the careers URL, e.g. "blueground" in apply.workable.com/blueground`,
			},
			&cli.StringFlag{
				Name:  "keyword",
				Usage: "server-side keyword search over title and posting body text",
			},
			&cli.StringFlag{
				Name:  "country",
				Usage: `country name as the filters facets list it, e.g. "Greece"`,
			},
			&cli.StringFlag{
				Name:  "region",
				Usage: `state/region name, e.g. "Attica"`,
			},
			&cli.StringFlag{
				Name:  "city",
				Usage: "city name",
			},
			&cli.IntFlag{
				Name:  "department-id",
				Usage: "numeric department id from 'openings-cli workable filters' (not the display name)",
			},
			&cli.StringFlag{
				Name:  "workplace",
				Usage: "on_site, hybrid, or remote",
			},
			&cli.StringFlag{
				Name:  "worktype",
				Usage: "full, part, contract, or temporary",
			},
			&cli.BoolFlag{
				Name:  "remote",
				Usage: "keep only remote jobs; --remote=false keeps only non-remote ones. Omit to not filter on it at all",
			},
			&cli.StringFlag{
				Name:  "page-token",
				Usage: "nextPage cursor from the previous page (page size is a fixed 10)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := rejectArgs(cmd); err != nil {
				return err
			}
			return runSearch(ctx, cmd)
		},
	}
}

// runSearch maps every flag onto the job board API's real server-side
// filters. Pagination is cursor-only: rerun with --page-token set to the
// previous output's nextToken.
func runSearch(ctx context.Context, cmd *cli.Command) error {
	account, err := normalizeCompany(cmd.String("company"))
	if err != nil {
		return err
	}

	workplace := cmd.String("workplace")
	if workplace != "" && !slices.Contains([]string{"on_site", "hybrid", "remote"}, workplace) {
		return fmt.Errorf("--workplace must be on_site, hybrid, or remote, got %q", workplace)
	}
	departmentID := cmd.Int("department-id")
	if departmentID < 0 {
		return fmt.Errorf("--department-id must be a positive facet id, got %d", departmentID)
	}

	ctx, cancel := context.WithTimeout(ctx, cmd.Duration("timeout"))
	defer cancel()

	client, err := newClient(cmd)
	if err != nil {
		return err
	}

	req := provider.SearchRequest{}
	if keyword := cmd.String("keyword"); keyword != "" {
		req.Query = provider.NewOptString(keyword)
	}
	country, region, city := cmd.String("country"), cmd.String("region"), cmd.String("city")
	if country != "" || region != "" || city != "" {
		loc := provider.LocationFilter{}
		if country != "" {
			loc.Country = provider.NewOptString(country)
		}
		if region != "" {
			loc.Region = provider.NewOptString(region)
		}
		if city != "" {
			loc.City = provider.NewOptString(city)
		}
		req.Location = []provider.LocationFilter{loc}
	}
	if departmentID > 0 {
		req.Department = []int{departmentID}
	}
	if workplace != "" {
		req.Workplace = []provider.SearchRequestWorkplaceItem{provider.SearchRequestWorkplaceItem(workplace)}
	}
	if worktype := cmd.String("worktype"); worktype != "" {
		req.Worktype = []string{worktype}
	}
	if cmd.IsSet("remote") {
		req.Remote = []string{fmt.Sprintf("%t", cmd.Bool("remote"))}
	}
	if pageToken := cmd.String("page-token"); pageToken != "" {
		req.Token = provider.NewOptString(pageToken)
	}

	res, err := client.SearchJobs(ctx, &req, provider.SearchJobsParams{Account: account})
	if err != nil {
		return err
	}

	page, ok := res.(*provider.SearchResponse)
	if !ok {
		return fmt.Errorf("company %q not found on Workable (account removed?)", account)
	}

	return json.NewEncoder(cmd.Root().Writer).Encode(page)
}
