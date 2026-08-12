package adp_myjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Default public hosts for ADP MyJobs.
const (
	DefaultCareerSiteBase = "https://myjobs.adp.com"
	DefaultListingBase    = "https://my.adp.com"
)

const (
	DefaultPageSize      = 100
	DefaultPageSpacing   = 250 * time.Millisecond
	listingPath          = "/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters"
	searchMetaPathPrefix = "/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/search-meta/"
	careerSitePathPrefix = "/public/staffing/v1/career-site/"
	defaultSelect        = "reqId,jobTitle,publishedJobTitle,type,jobDescription,jobQualifications,workLevelCode,clientRequisitionID,postingDate,requisitionLocations"
	defaultOrderBy       = "postingDate desc"
	defaultTZ            = "America/New_York"
)

// Config configures a Client. Zero values pick production defaults.
type Config struct {
	HTTPClient     *http.Client
	CareerSiteBase string // default DefaultCareerSiteBase
	ListingBase    string // default DefaultListingBase
	PageSize       int
	PageSpacing    time.Duration // 0 = default; <0 = no delay (tests)
}

// ListParams is one page of public job requisitions.
type ListParams struct {
	// Search is sent as OData $search (server-side free-text over the posting).
	// Empty means no $search.
	Search string
	// GeoBox restricts results to postings located inside the box, sent as the
	// endpoint's one supported $filter form. nil means no $filter.
	GeoBox *GeoBox
	Skip   int
	Top    int
}

// GeoBox is a longitude/latitude bounding box, the only location constraint the
// public listing endpoint honors. It matches against workLocations.geoLocation,
// which is indexed per posting location at street precision even on boards
// whose listing payload returns workLocations empty.
//
// Everything else callers might reach for is a trap: the endpoint answers a
// $filter over any other field — requisitionLocations/itemID, workLocations.city,
// or a misspelled field name — with HTTP 200 and the unfiltered board, so a
// wrong expression reads as "this company has no location filter" rather than
// as an error. Only one box per request is possible; ORing two boxes is either
// rejected (HTTP 500) or silently ignored, and a polygon with more than the two
// corners below is rejected.
type GeoBox struct {
	West, South, East, North float64
}

// NewGeoBox returns the box spanning the two corners, ordered as the endpoint
// requires. Callers pass corners in any order; a box needs west < east and
// south < north, since inverted longitudes quietly select a different region
// and a zero-area box is answered with HTTP 500.
func NewGeoBox(lon1, lat1, lon2, lat2 float64) GeoBox {
	return GeoBox{
		West:  min(lon1, lon2),
		South: min(lat1, lat2),
		East:  max(lon1, lon2),
		North: max(lat1, lat2),
	}
}

// Pad grows the box by delta degrees on all four sides. A box built from a
// single posting location has zero area, which upstream rejects, so callers
// resolving a location to coordinates must pad before sending.
//
// Corners are rounded to coordDecimals places, so the float noise of adding
// delta does not reach the query string.
func (b GeoBox) Pad(delta float64) GeoBox {
	return GeoBox{
		West:  roundCoord(b.West - delta),
		South: roundCoord(b.South - delta),
		East:  roundCoord(b.East + delta),
		North: roundCoord(b.North + delta),
	}
}

// coordDecimals is the precision kept for box corners: 7 decimal places is
// ~1 cm, far finer than the per-location coordinates upstream publishes.
const coordDecimals = 7

func roundCoord(v float64) float64 {
	scale := math.Pow(10, coordDecimals)
	return math.Round(v*scale) / scale
}

// Filter renders b as an OData $filter value.
//
// The literal "undefined" tokens are required, not a bug being copied for its
// own sake: they come from a broken template in ADP's own career SPA, and the
// endpoint's parser now only accepts that shape. A well-formed
// POLYGON((lon lat, lon lat)) with the same corners is answered with HTTP 500.
func (b GeoBox) Filter() string {
	return "geo.intersects(workLocations.geoLocation, geography'POLYGON((undefined, " +
		formatCoord(b.West) + " " + formatCoord(b.South) + ", undefined, " +
		formatCoord(b.East) + " " + formatCoord(b.North) + "))')"
}

// formatCoord renders a coordinate in plain decimal notation, never the
// exponent form Go's default float formatting would pick near zero, which is
// not valid inside a WKT literal.
func formatCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// Client talks to public MyJobs career-site and listing endpoints.
type Client struct {
	hc             *http.Client
	careerSiteBase string
	listingBase    string
	pageSize       int
	pageSpacing    time.Duration

	mu     sync.Mutex
	tokens map[string]session // slug -> public session
}

type session struct {
	token  string
	orgOID string
}

// NewClient builds a MyJobs public client from cfg.
func NewClient(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	career := strings.TrimRight(cfg.CareerSiteBase, "/")
	if career == "" {
		career = DefaultCareerSiteBase
	}
	listing := strings.TrimRight(cfg.ListingBase, "/")
	if listing == "" {
		listing = DefaultListingBase
	}
	pageSize := cfg.PageSize
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	pageSpacing := cfg.PageSpacing
	switch {
	case pageSpacing < 0:
		pageSpacing = 0
	case pageSpacing == 0:
		pageSpacing = DefaultPageSpacing
	}
	return &Client{
		hc:             hc,
		careerSiteBase: career,
		listingBase:    listing,
		pageSize:       pageSize,
		pageSpacing:    pageSpacing,
		tokens:         make(map[string]session),
	}
}

// GetCareerSite fetches public career-site config for slug.
func (c *Client) GetCareerSite(ctx context.Context, slug string) (*CareerSite, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return nil, fmt.Errorf("adp_myjobs: empty slug")
	}
	u := c.careerSiteBase + careerSitePathPrefix + url.PathEscape(slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adp_myjobs: career-site %q: %w", slug, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("adp_myjobs: career-site %q: HTTP %d", slug, res.StatusCode)
	}
	var cs CareerSite
	if err := json.NewDecoder(res.Body).Decode(&cs); err != nil {
		return nil, fmt.Errorf("adp_myjobs: career-site %q decode: %w", slug, err)
	}
	if cs.MyJobsToken == "" {
		return nil, fmt.Errorf("adp_myjobs: career-site %q: missing myJobsToken", slug)
	}
	c.mu.Lock()
	c.tokens[slug] = session{token: cs.MyJobsToken, orgOID: cs.OrgOID}
	c.mu.Unlock()
	return &cs, nil
}

// ListJobRequisitions fetches one page of requisitions.
// Non-empty Search is sent as $search (server-side free-text keyword).
func (c *Client) ListJobRequisitions(ctx context.Context, slug string, p ListParams) (*ListResult, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if p.Top <= 0 {
		p.Top = c.pageSize
	}
	if p.Skip < 0 {
		p.Skip = 0
	}
	sess, err := c.ensureSession(ctx, slug)
	if err != nil {
		return nil, err
	}
	return c.listOnce(ctx, slug, sess, p)
}

// ListAllJobRequisitions fully paginates a board.
func (c *Client) ListAllJobRequisitions(ctx context.Context, slug string) ([]JobRequisition, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	var (
		all   []JobRequisition
		skip  int
		total = -1
	)
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("adp_myjobs: list all %q: %w", slug, err)
		}
		page, err := c.ListJobRequisitions(ctx, slug, ListParams{Skip: skip})
		if err != nil {
			return nil, err
		}
		if total < 0 {
			total = page.Count
		}
		all = append(all, page.JobRequisitions...)
		if len(page.JobRequisitions) == 0 || len(all) >= total {
			return all, nil
		}
		skip += len(page.JobRequisitions)
		if c.pageSpacing <= 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.pageSpacing):
		}
	}
}

// GetJobRequisition fetches one requisition by reqId via search-meta.
func (c *Client) GetJobRequisition(ctx context.Context, slug, reqID string) (*JobRequisition, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	reqID = strings.TrimSpace(reqID)
	if slug == "" || reqID == "" {
		return nil, fmt.Errorf("adp_myjobs: slug and reqId are required")
	}
	sess, err := c.ensureSession(ctx, slug)
	if err != nil {
		return nil, err
	}
	u := c.listingBase + searchMetaPathPrefix + url.PathEscape(reqID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.setListingHeaders(req, sess)
	req.Header.Set("Accept-Language", "en-US")
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adp_myjobs: detail %q for %q: %w", reqID, slug, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("adp_myjobs: detail %q for %q: HTTP %d", reqID, slug, res.StatusCode)
	}
	var raw searchMetaResult
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("adp_myjobs: detail %q for %q decode: %w", reqID, slug, err)
	}
	if len(raw.JobRequisitions) == 0 {
		return nil, fmt.Errorf("adp_myjobs: job %q not found for %q", reqID, slug)
	}
	j := raw.JobRequisitions[0].toJobRequisition(reqID)
	return &j, nil
}

func (c *Client) ensureSession(ctx context.Context, slug string) (session, error) {
	c.mu.Lock()
	s, ok := c.tokens[slug]
	c.mu.Unlock()
	if ok && s.token != "" {
		return s, nil
	}
	cs, err := c.GetCareerSite(ctx, slug)
	if err != nil {
		return session{}, err
	}
	return session{token: cs.MyJobsToken, orgOID: cs.OrgOID}, nil
}

func (c *Client) setListingHeaders(req *http.Request, sess session) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://myjobs.adp.com")
	req.Header.Set("Referer", "https://myjobs.adp.com/")
	req.Header.Set("myjobstoken", sess.token)
	req.Header.Set("rolecode", "manager")
	if sess.orgOID != "" {
		req.Header.Set("orgoid", sess.orgOID)
	}
}

// encodeQuery encodes q with spaces as %20 rather than "+".
//
// The two are interchangeable to this endpoint everywhere except inside a geo
// $filter, where a "+" makes it answer HTTP 500 — url.Values.Encode alone is
// enough to break every location search. Replacing "+" afterwards is safe
// because Encode has already escaped any literal plus as %2B.
func encodeQuery(q url.Values) string {
	return strings.ReplaceAll(q.Encode(), "+", "%20")
}

func (c *Client) listOnce(ctx context.Context, slug string, sess session, p ListParams) (*ListResult, error) {
	q := url.Values{}
	q.Set("$orderby", defaultOrderBy)
	q.Set("$select", defaultSelect)
	q.Set("$top", strconv.Itoa(p.Top))
	q.Set("$skip", strconv.Itoa(p.Skip))
	q.Set("tz", defaultTZ)
	if s := strings.TrimSpace(p.Search); s != "" {
		q.Set("$search", s)
	}
	if p.GeoBox != nil {
		q.Set("$filter", p.GeoBox.Filter())
	}
	u := c.listingBase + listingPath + "?" + encodeQuery(q)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.setListingHeaders(req, sess)

	res, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adp_myjobs: list %q: %w", slug, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("adp_myjobs: list %q: HTTP %d", slug, res.StatusCode)
	}
	var out ListResult
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("adp_myjobs: list %q decode: %w", slug, err)
	}
	return &out, nil
}
