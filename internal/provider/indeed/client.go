package indeed

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Khan/genqlient/graphql"
)

// mobileAppKey is baked into the official Indeed mobile app's binary, not a
// per-caller secret: it identifies "a request from the Indeed app," the same
// way a Google Maps or Firebase client key does, and is recoverable by
// anyone who decompiles the app or inspects its traffic — which is how
// python-jobspy's constant.py (and this client) got it. See openapi.yaml's
// Key Behaviors for why this is not a value to invent or rotate yourself.
const mobileAppKey = "161092c2017b5bbab13edb12461a62d5a833871e7cad6d9d475304573de67ac8"

const userAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Indeed App 193.1"
const appInfo = "appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone"

type countryCtxKey struct{}

// Client talks to Indeed's mobile GraphQL endpoint via genqlient.
type Client struct {
	gql graphql.Client
}

// NewClient builds a Client against apiURL (the GraphQL endpoint). When
// httpClient is nil, http.DefaultClient is used. Every request carries the
// static mobile-app headers; per-call indeed-co is taken from context set
// inside Jobs / JobDetail.
func NewClient(apiURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	// Copy so we don't mutate the caller's client (httptest's, shared pools, etc.).
	hc := *httpClient
	base := hc.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	hc.Transport = &indeedTransport{base: base}
	return &Client{gql: graphql.NewClient(apiURL, &hc)}
}

// indeedTransport injects the static mobile-app headers and the
// per-request indeed-co country code stashed on the context.
type indeedTransport struct {
	base http.RoundTripper
}

func (t *indeedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	req.Header.Set("indeed-locale", "en-US")
	req.Header.Set("user-agent", userAgent)
	req.Header.Set("indeed-app-info", appInfo)
	req.Header.Set("indeed-api-key", mobileAppKey)
	if co, ok := req.Context().Value(countryCtxKey{}).(string); ok && co != "" {
		req.Header.Set("indeed-co", co)
	}
	return t.base.RoundTrip(req)
}

// resolveCountry looks up r's Country (defaulting to DefaultCountryName),
// reporting an error for a name CountryByName doesn't recognize.
func resolveCountry(name string) (Country, error) {
	if name == "" {
		name = DefaultCountryName
	}
	c, ok := CountryByName(name)
	if !ok {
		return Country{}, fmt.Errorf("unknown country %q", name)
	}
	return c, nil
}

// siteBaseURL builds the country-specific Indeed website's base URL, used
// for job/company links (distinct from the GraphQL API host, which is the
// same apis.indeed.com regardless of country).
func siteBaseURL(c Country) string {
	return "https://" + c.Domain + ".indeed.com"
}

func withCountry(ctx context.Context, co string) context.Context {
	return context.WithValue(ctx, countryCtxKey{}, co)
}

// Jobs searches jobSearch with r's criteria.
func (c *Client) Jobs(ctx context.Context, r *JobsRequest) (*JobsResponse, error) {
	country, err := resolveCountry(r.Country)
	if err != nil {
		return nil, err
	}
	what := r.Keywords
	var location *JobSearchLocationInput
	if r.Location != "" {
		radius := r.RadiusMiles
		if radius == 0 {
			radius = 25
		}
		location = &JobSearchLocationInput{
			Where:      r.Location,
			Radius:     radius,
			RadiusUnit: RadiusUnitMiles,
		}
	}
	limit := r.Limit
	if limit == 0 {
		limit = 25
	}
	wire, err := GetJobSearch(
		withCountry(ctx, country.APICode),
		c.gql,
		what,
		location,
		limit,
		JobSearchSortOrderRelevance,
		r.Cursor,
		searchFilters(r),
	)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	if wire.JobSearch == nil {
		return &JobsResponse{}, nil
	}
	base := siteBaseURL(country)
	jobs := make([]Job, 0, len(wire.JobSearch.Results))
	for _, result := range wire.JobSearch.Results {
		jobs = append(jobs, jobFromSearch(result.Job, base))
	}
	resp := &JobsResponse{Jobs: jobs}
	if wire.JobSearch.PageInfo.NextCursor != nil {
		resp.NextCursor = *wire.JobSearch.PageInfo.NextCursor
	}
	return resp, nil
}

// searchFilters builds the jobSearch filters list. The live API takes
// filters: [JobSearchFilterInput!]! — always a list (empty when none).
// Precedence matches python-jobspy's _build_filters and openapi.yaml:
// HoursOld, then EasyApply, then JobType/Remote.
func searchFilters(r *JobsRequest) []JobSearchFilterInput {
	switch {
	case r.HoursOld > 0:
		return []JobSearchFilterInput{{
			Date: &DateFilterInput{
				Field: "dateOnIndeed",
				Start: fmt.Sprintf("%dh", r.HoursOld),
			},
		}}
	case r.EasyApply:
		return []JobSearchFilterInput{{
			Keyword: &KeywordFilterInput{
				Field: "indeedApplyScope",
				Keys:  []string{"DESKTOP"},
			},
		}}
	case r.JobType != "" || r.Remote:
		keys := make([]string, 0, 2)
		if r.JobType != "" {
			keys = append(keys, r.JobType)
		}
		if r.Remote {
			keys = append(keys, remoteAttributeKey)
		}
		return []JobSearchFilterInput{{
			Composite: &CompositeFilterInput{
				Filters: []JobSearchFilterInput{{
					Keyword: &KeywordFilterInput{
						Field: "attributes",
						Keys:  keys,
					},
				}},
			},
		}}
	default:
		// Non-null list arg: send [] not null.
		return []JobSearchFilterInput{}
	}
}

// JobDetail looks up one job by its key (Job.Key from a prior Jobs call).
// A key with no matching job (removed, expired, or never valid) returns
// (nil, nil) rather than an error — see openapi.yaml's Key Behaviors on
// jobData's empty-list-not-404 shape.
func (c *Client) JobDetail(ctx context.Context, country, jobKey string) (*JobDetail, error) {
	if jobKey == "" {
		return nil, errors.New("empty job key")
	}
	resolved, err := resolveCountry(country)
	if err != nil {
		return nil, err
	}
	wire, err := GetJobDetail(
		withCountry(ctx, resolved.APICode),
		c.gql,
		[]string{jobKey},
	)
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", jobKey, err)
	}
	if wire.JobData == nil || len(wire.JobData.Results) == 0 {
		return nil, nil
	}
	detail := jobDetailFromDetail(wire.JobData.Results[0].Job, siteBaseURL(resolved))
	return &detail, nil
}
