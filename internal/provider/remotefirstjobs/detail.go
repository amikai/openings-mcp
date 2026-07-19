package remotefirstjobs

import (
	"context"
	"fmt"
)

// The API serves at most maxPage+1 pages of pageSize jobs per query —
// both limits come from the official docs at
// https://remotefirstjobs.com/jobs-api.
const (
	pageSize = 100
	maxPage  = 4
)

// FindOptions narrows [Client.FindJob]'s page scan to the search that
// surfaced the id, cutting the requests needed to reach it. Zero-valued
// fields don't narrow.
type FindOptions struct {
	// Query is the full-text term the job was found under.
	Query string
	// Category is the job's category (a search hit's category field
	// round-trips as a valid filter value).
	Category string
}

// FindJob resolves one job by [Job.ID]. There is no per-job endpoint —
// see openapi.yaml — so this scans search pages until the id appears,
// stopping early on a short page. Only the newest 500 jobs per query are
// reachable; an id outside that window (or expired) is an error, which
// opts can avoid by narrowing the scan to the search that surfaced it.
func (c *Client) FindJob(ctx context.Context, id string, opts FindOptions) (*Job, error) {
	params := SearchJobsParams{}
	if opts.Query != "" {
		params.Query = NewOptString(opts.Query)
	}
	if opts.Category != "" {
		params.Category = NewOptString(opts.Category)
	}
	for page := 0; page <= maxPage; page++ {
		params.Page = NewOptInt(page)
		res, err := c.SearchJobs(ctx, params)
		if err != nil {
			return nil, err
		}
		result, ok := res.(*SearchJobsResult)
		if !ok {
			return nil, fmt.Errorf("search page %d: %w", page, res.(*Error))
		}
		for i := range result.Jobs {
			if result.Jobs[i].ID == id {
				return &result.Jobs[i], nil
			}
		}
		if len(result.Jobs) < pageSize {
			break
		}
	}
	return nil, fmt.Errorf("job %q not found in the reachable search pages; it may have expired or fallen outside the newest 500", id)
}

// Error implements the error interface so a 400 response can be
// returned directly as a Go error.
func (e *Error) Error() string {
	return fmt.Sprintf("remotefirstjobs API error (%s): %s", e.Kind, e.Message)
}
