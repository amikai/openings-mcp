package eightfold

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// FilteredSearch is the input to SearchFiltered: transport, tenant base URL,
// typed search params, and resolved facet filter values.
type FilteredSearch struct {
	HTTPClient *http.Client
	BaseURL    string
	Params     SearchParams
	Filters    map[string][]string
}

// SearchFiltered issues a search request with server-side facet filters
// applied, bypassing the generated Client. Facet filters are sent as
// filter_<facetName>=<value> query parameters (repeatable per value for OR
// semantics), and facetName is tenant-specific — discovered at runtime
// from an unfiltered search response's data.filterDef.smartFilters, never
// known ahead of time. OpenAPI has no way to declare a dynamically-named
// parameter, so this builds the request by hand and decodes into the same
// generated SearchResponse type the typed client uses.
func SearchFiltered(ctx context.Context, r FilteredSearch) (*SearchResponse, error) {
	hc := r.HTTPClient
	baseURL := r.BaseURL
	params := r.Params
	filters := r.Filters
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("eightfold: parse base URL %q: %w", baseURL, err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/pcsx/search"

	q := u.Query()
	q.Set("domain", params.Domain)
	q.Set("query", params.Query.Or(""))
	q.Set("location", params.Location.Or(""))
	q.Set("start", strconv.Itoa(params.Start.Or(0)))
	for name, values := range filters {
		for _, v := range values {
			q.Add("filter_"+name, v)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("eightfold: build search request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	if hc == nil {
		hc = http.DefaultClient
	}
	rsp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eightfold: search request: %w", err)
	}
	defer rsp.Body.Close()

	// A domain that doesn't match the tenant's registered value gets the
	// site's HTML shell back, not JSON — surfaced here as a decode error
	// rather than silently returned as an empty result.
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eightfold: search returned HTTP %d", rsp.StatusCode)
	}

	var out SearchResponse
	if err := json.NewDecoder(rsp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("eightfold: decode search response: %w", err)
	}
	return &out, nil
}
