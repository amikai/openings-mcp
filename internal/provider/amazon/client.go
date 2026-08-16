package amazon

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	defaultResultLimit = 10
	maxResultLimit     = 100
)

// ErrJobNotFound reports an Amazon posting ID that has no exact live match.
var ErrJobNotFound = errors.New("amazon: job not found")

// SearchRequest contains the supported Amazon Jobs search filters. Slice
// values are OR'd within one filter and AND'd across different filters.
type SearchRequest struct {
	Query              string
	Countries          []string
	Cities             []string
	JobCategories      []string
	BusinessCategories []string
	ScheduleTypes      []string
	Sort               SearchJobsSort
	Offset             int
	Limit              int
}

// SearchResult contains one page and the total number of matching jobs.
type SearchResult struct {
	Total int
	Jobs  []Job
}

// Search applies defaults and rejects values that the upstream otherwise
// ignores or reports through an HTTP-200 soft-error payload.
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if req.Offset < 0 {
		return nil, fmt.Errorf("offset must be at least 0, got %d", req.Offset)
	}

	limit := req.Limit
	if limit == 0 {
		limit = defaultResultLimit
	}
	if limit < 1 || limit > maxResultLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d, got %d", maxResultLimit, limit)
	}

	sort := req.Sort
	if sort == "" {
		sort = SearchJobsSortRelevant
	}
	if err := sort.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sort %q: %w", sort, err)
	}

	params := SearchJobsParams{
		NormalizedCountryCode: append([]string{}, req.Countries...),
		NormalizedCityName:    append([]string{}, req.Cities...),
		Category:              append([]string{}, req.JobCategories...),
		BusinessCategory:      append([]string{}, req.BusinessCategories...),
		ScheduleTypeID:        append([]string{}, req.ScheduleTypes...),
		Sort:                  NewOptSearchJobsSort(sort),
		Offset:                NewOptInt(req.Offset),
		ResultLimit:           NewOptInt(limit),
	}
	if req.Query != "" {
		params.BaseQuery = NewOptString(req.Query)
	}

	response, err := c.SearchJobs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	if upstreamError, ok := response.Error.Get(); ok && upstreamError != "" {
		return nil, fmt.Errorf("search jobs: upstream rejected request: %s", strings.ToLower(upstreamError))
	}

	jobs, ok := response.Jobs.Get()
	if !ok {
		return nil, errors.New("search jobs: response omitted jobs")
	}
	if jobs == nil {
		jobs = []Job{}
	}
	return &SearchResult{Total: response.Hits, Jobs: jobs}, nil
}

// JobDetail returns the exact live posting whose public numeric ID matches
// jobID. Amazon exposes no separate JSON detail operation, so this uses the
// search endpoint and verifies the returned ID before accepting the result.
func (c *Client) JobDetail(ctx context.Context, jobID string) (*Job, error) {
	if !isNumericID(jobID) {
		return nil, fmt.Errorf("invalid job id %q: expected digits only", jobID)
	}

	result, err := c.Search(ctx, SearchRequest{Query: jobID})
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", jobID, err)
	}
	for i := range result.Jobs {
		if result.Jobs[i].IDIcims == jobID {
			return &result.Jobs[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrJobNotFound, jobID)
}

func isNumericID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// JobURL converts a job_path from the API into its public posting URL.
func JobURL(jobPath string) string {
	if !strings.HasPrefix(jobPath, "/") {
		return ""
	}
	return "https://www.amazon.jobs" + jobPath
}
