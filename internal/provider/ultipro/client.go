// Package ultipro is a client for public UltiPro (UKG Pro Recruiting)
// career-board endpoints, documented in openapi.yaml. There is no ogen
// codegen here: LoadSearchResults/GetFilters/ViewMore* are clean JSON, but
// job detail is server-rendered HTML with the posting embedded as a JSON
// object literal, the same "JSON smuggled inside HTML" situation that
// keeps internal/provider/icims and internal/provider/successfactors on a
// hand-written client too.
package ultipro

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrCompanyNotFound indicates an unknown companyCode or boardId (HTTP 404
// on any JobBoardView/JobBoardViewMore endpoint).
var ErrCompanyNotFound = errors.New("ultipro: company not found")

// ErrJobNotFound indicates opportunityId did not resolve to a live
// posting: OpportunityDetail answers HTTP 200 with the bare app shell and
// no embedded CandidateOpportunityDetail payload (see openapi.yaml).
var ErrJobNotFound = errors.New("ultipro: job not found")

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Client talks to one tenant board at baseURL (e.g.
// "https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24",
// see [Company.BaseURL] / [CareersSite.BaseURL]).
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a client for one tenant board's baseURL.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// SearchFilter is one entry of LoadSearchResults' Filters array.
// FieldName 4 = physical location id, 5 = job category id, 37 = job
// location type (0=Hybrid, 1=On-site, 2=Remote). FieldName 6 (schedule)
// is deliberately never sent — see openapi.yaml's Key Behaviors.
type SearchFilter struct {
	FieldName int
	Values    []string
}

// SearchRequest mirrors LoadSearchResults' opportunitySearch object.
type SearchRequest struct {
	Query   string
	Top     int
	Skip    int
	Filters []SearchFilter
}

// Opportunity is one LoadSearchResults result row.
type Opportunity struct {
	ID                string     `json:"Id"`
	Title             string     `json:"Title"`
	RequisitionNumber string     `json:"RequisitionNumber"`
	JobCategoryName   string     `json:"JobCategoryName"`
	Locations         []Location `json:"Locations"`
	PostedDate        string     `json:"PostedDate"`
	BriefDescription  string     `json:"BriefDescription"`
	JobLocationType   *int       `json:"JobLocationType"`
}

// Location is one posting or catalog location entry.
type Location struct {
	ID                   string  `json:"Id"`
	LocalizedName        string  `json:"LocalizedName"`
	LocalizedDescription string  `json:"LocalizedDescription"`
	Address              Address `json:"Address"`
}

type Address struct {
	City    string     `json:"City"`
	State   *NamedCode `json:"State"`
	Country *NamedCode `json:"Country"`
}

type NamedCode struct {
	Code string `json:"Code"`
	Name string `json:"Name"`
}

// Display renders a location the way the public board does: prefer
// LocalizedDescription (verified live — e.g. "Ethiopia/Aleta Wondo" is
// what the search-results card shows for a location whose LocalizedName
// is null), then LocalizedName, then City/State/Country composed from
// Address for locations carrying neither.
func (l Location) Display() string {
	if l.LocalizedDescription != "" {
		return l.LocalizedDescription
	}
	if l.LocalizedName != "" {
		return l.LocalizedName
	}
	var parts []string
	if l.Address.City != "" {
		parts = append(parts, l.Address.City)
	}
	if l.Address.State != nil && l.Address.State.Name != "" {
		parts = append(parts, l.Address.State.Name)
	}
	if l.Address.Country != nil && l.Address.Country.Name != "" {
		parts = append(parts, l.Address.Country.Name)
	}
	return strings.Join(parts, ", ")
}

// SearchResponse is LoadSearchResults' response body.
type SearchResponse struct {
	Opportunities []Opportunity `json:"opportunities"`
	TotalCount    int           `json:"totalCount"`
}

// Search runs one LoadSearchResults call. Sorting is always PostedDate
// descending for predictable pagination (see openapi.yaml).
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	filters := make([]searchFilterWire, 0, len(req.Filters))
	for _, f := range req.Filters {
		values := f.Values
		if values == nil {
			values = []string{}
		}
		filters = append(filters, searchFilterWire{T: "TermsSearchFilterDto", FieldName: f.FieldName, Values: values})
	}
	body := loadSearchResultsWire{
		OpportunitySearch: opportunitySearchWire{
			Top:         req.Top,
			Skip:        req.Skip,
			QueryString: req.Query,
			OrderBy: []orderByWire{
				{Value: "postedDateDesc", PropertyName: "PostedDate", Ascending: false},
			},
			Filters: filters,
		},
		MatchCriteria: matchCriteriaWire{
			PreferredJobs:            []any{},
			Educations:               []any{},
			LicenseAndCertifications: []any{},
			Skills:                   []any{},
			SkippedSkills:            []any{},
		},
	}

	var out SearchResponse
	if err := c.postJSON(ctx, "/JobBoardView/LoadSearchResults", body, &out); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return &out, nil
}

// FilterCatalog is a ViewMore catalog entry: an id usable in a
// SearchFilter and its display label.
type FilterCatalog struct {
	ID    string
	Label string
}

// Locations fetches the board's full physical-location catalog (fieldName
// 4 filter values). Top:500 comfortably covers any observed tenant (the
// largest confirmed catalog has 44 entries) in one call — see openapi.yaml.
func (c *Client) Locations(ctx context.Context) ([]FilterCatalog, error) {
	var out struct {
		Locations []Location `json:"locations"`
	}
	if err := c.postJSON(ctx, "/JobBoardViewMore/ViewMorePhysicalLocations", viewMoreWire{Top: 500}, &out); err != nil {
		return nil, fmt.Errorf("locations: %w", err)
	}
	catalog := make([]FilterCatalog, 0, len(out.Locations))
	for _, l := range out.Locations {
		if l.ID == "" {
			continue
		}
		catalog = append(catalog, FilterCatalog{ID: l.ID, Label: l.Display()})
	}
	return catalog, nil
}

// Categories fetches the board's full job-category catalog (fieldName 5
// filter values).
func (c *Client) Categories(ctx context.Context) ([]FilterCatalog, error) {
	var out struct {
		Categories []struct {
			ID          string `json:"Id"`
			DisplayName string `json:"DisplayName"`
		} `json:"categories"`
	}
	if err := c.postJSON(ctx, "/JobBoardViewMore/ViewMoreJobCategories", viewMoreWire{Top: 500}, &out); err != nil {
		return nil, fmt.Errorf("categories: %w", err)
	}
	catalog := make([]FilterCatalog, 0, len(out.Categories))
	for _, cat := range out.Categories {
		if cat.ID == "" || cat.DisplayName == "" {
			continue
		}
		catalog = append(catalog, FilterCatalog{ID: cat.ID, Label: cat.DisplayName})
	}
	return catalog, nil
}

// OpportunityDetail is the object embedded in OpportunityDetail's HTML.
type OpportunityDetail struct {
	ID                string     `json:"Id"`
	Title             string     `json:"Title"`
	RequisitionNumber string     `json:"RequisitionNumber"`
	JobCategoryName   string     `json:"JobCategoryName"`
	Locations         []Location `json:"Locations"`
	PostedDate        string     `json:"PostedDate"`
	Description       string     `json:"Description"`
}

// Detail fetches and parses one job's OpportunityDetail page.
func (c *Client) Detail(ctx context.Context, opportunityID string) (*OpportunityDetail, error) {
	u := c.baseURL + "/OpportunityDetail?opportunityId=" + opportunityID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("detail: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("detail: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("detail: read body: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("detail: %w", ErrCompanyNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("detail: HTTP %d", resp.StatusCode)
	}

	detail, ok := extractOpportunityDetail(body)
	if !ok {
		return nil, fmt.Errorf("detail: %w", ErrJobNotFound)
	}
	return detail, nil
}

func (c *Client) postJSON(ctx context.Context, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrCompanyNotFound
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(data))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// Wire types for LoadSearchResults' request body. Kept private and
// separate from the public Opportunity/Location response types since the
// request shape (nested opportunitySearch/matchCriteria, PascalCase
// scalar keys) is unrelated to what comes back.
type loadSearchResultsWire struct {
	OpportunitySearch opportunitySearchWire `json:"opportunitySearch"`
	MatchCriteria     matchCriteriaWire     `json:"matchCriteria"`
}

type opportunitySearchWire struct {
	Top         int                `json:"Top"`
	Skip        int                `json:"Skip"`
	QueryString string             `json:"QueryString"`
	OrderBy     []orderByWire      `json:"OrderBy"`
	Filters     []searchFilterWire `json:"Filters"`
}

type orderByWire struct {
	Value        string `json:"Value"`
	PropertyName string `json:"PropertyName"`
	Ascending    bool   `json:"Ascending"`
}

type searchFilterWire struct {
	T         string   `json:"t"`
	FieldName int      `json:"fieldName"`
	Values    []string `json:"values"`
}

type matchCriteriaWire struct {
	PreferredJobs            []any `json:"PreferredJobs"`
	Educations               []any `json:"Educations"`
	LicenseAndCertifications []any `json:"LicenseAndCertifications"`
	Skills                   []any `json:"Skills"`
	HasNoLicenses            bool  `json:"hasNoLicenses"`
	SkippedSkills            []any `json:"SkippedSkills"`
}

// viewMoreWire is ViewMorePhysicalLocations/ViewMoreJobCategories' request
// body. Top/Skip are top-level here, unlike LoadSearchResults — see
// openapi.yaml.
type viewMoreWire struct {
	Top  int `json:"Top"`
	Skip int `json:"Skip"`
}
