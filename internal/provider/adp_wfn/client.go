package adp_wfn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the public career-center API root. It carries no version
// segment because one operation (client-features) sits outside the /v1 prefix
// the others use.
const DefaultBaseURL = "https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing"

// DefaultLegacyBaseURL is the host serving the retired career center, whose
// posting.html still resolves a readable client slug to a tenant GUID.
const DefaultLegacyBaseURL = "https://workforcenow.adp.com"

// PageSize is the only page size worth sending. The upstream caps $top at 20
// and ignores it outright once any filter is applied, so a caller stepping by
// anything smaller silently refetches rows it already has.
const PageSize = 20

// legacyResolveTimeout bounds the redirect chain that turns a legacy client
// slug into a tenant GUID. Measured 1.19-1.54s over repeated runs, so this
// leaves roughly two-fold headroom; the path is only ever reached for a URL
// form ADP retired in June 2026.
const legacyResolveTimeout = 3 * time.Second

// ErrTenantNotFound reports a cid that no career center answers to. The
// upstream signals it with HTTP 404 and an openresty HTML body.
var ErrTenantNotFound = errors.New("adp_wfn: tenant not found")

// ErrJobNotFound reports a posting id the tenant does not publish. The
// upstream answers HTTP 200 with a record stripped of its itemID rather than
// a 404, so this is inferred rather than read off the status line.
var ErrJobNotFound = errors.New("adp_wfn: job requisition not found")

// Config configures a [BoardClient]. The two base URLs are separate because
// they address different services: the JSON API, and the retired career
// center that still performs slug resolution.
type Config struct {
	HTTPClient    *http.Client
	BaseURL       string
	LegacyBaseURL string
}

// BoardClient reads public Workforce Now career centers. It holds no session:
// every operation is a bare unauthenticated GET, and the one place a cookie is
// needed ([BoardClient.ResolveLegacySlug]) builds a throwaway jar rather than
// keeping one.
type BoardClient struct {
	api       *Client
	hc        *http.Client
	legacyURL string
}

// NewBoardClient builds a client against cfg, falling back to the public
// endpoints and http.DefaultClient for anything cfg leaves empty.
func NewBoardClient(cfg Config) (*BoardClient, error) {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	legacy := cfg.LegacyBaseURL
	if legacy == "" {
		legacy = DefaultLegacyBaseURL
	}
	api, err := NewClient(base, WithClient(hc))
	if err != nil {
		return nil, fmt.Errorf("adp_wfn: create api client: %w", err)
	}
	return &BoardClient{api: api, hc: hc, legacyURL: strings.TrimRight(legacy, "/")}, nil
}

// ListParams selects one page of a tenant's board.
//
// Query, Locations, and WorkerCategories compose as an AND upstream. Locations
// and WorkerCategories OR within themselves.
type ListParams struct {
	// Locale must be the tenant's own. Naming another one it recognizes
	// returns an empty board rather than an error.
	Locale string
	// Query is a relevance probe over the description text, and only the
	// first page of its results is meaningful; see [BoardClient.List].
	Query string
	// Locations must already be `<value>,<qualifier>` pairs. Sending a bare
	// token returns the whole unfiltered board, so callers validate before
	// they reach here.
	Locations []string
	// WorkerCategories must be oids published by [BoardClient.SearchFilters].
	// A label, or an oid from another tenant, returns the whole board.
	WorkerCategories []string
	// Page is 1-based and converted to the upstream's 1-based row offset.
	Page int
}

// filtered reports whether any control that makes meta.totalNumber unreliable
// and $top inert is in play.
func (p ListParams) filtered() bool {
	return strings.TrimSpace(p.Query) != "" || len(p.Locations) > 0 || len(p.WorkerCategories) > 0
}

// ListResult is one page of a board, plus whether the count that came with it
// can be believed.
type ListResult struct {
	Jobs []JobRequisition
	// TotalNumber is the upstream count. It is only a row count when
	// TotalTrusted is true; under a filter the upstream reports a relevance
	// tally that routinely exceeds the whole board.
	TotalNumber int
	// TotalTrusted reports whether the request was unfiltered.
	TotalTrusted bool
	// HasMore reports whether a full page came back, which is the only
	// evidence of a further page when the count cannot be trusted.
	HasMore bool
}

// List returns one page of tenant cid's board.
//
// Row offsets are 1-based upstream: asking for offset 0 silently drops a row.
// When p carries a Query, only page 1 is meaningful — the upstream orders
// relevance results nondeterministically between identical calls, so
// consecutive windows overlap and drop rows. Callers must not page a query.
func (c *BoardClient) List(ctx context.Context, cid string, p ListParams) (*ListResult, error) {
	page := max(p.Page, 1)
	params := ListJobRequisitionsParams{
		Cid:  cid,
		Skip: NewOptInt32(int32((page-1)*PageSize + 1)),
		Top:  NewOptInt32(PageSize),
	}
	if locale := strings.TrimSpace(p.Locale); locale != "" {
		params.Locale = NewOptString(locale)
	}
	if q := strings.TrimSpace(p.Query); q != "" {
		params.UserQuery = NewOptString(q)
	}
	if len(p.Locations) > 0 {
		params.LocationsList = NewOptString(strings.Join(p.Locations, ","))
	}
	if len(p.WorkerCategories) > 0 {
		params.WorkerCategoriesList = NewOptString(strings.Join(p.WorkerCategories, ","))
	}

	res, err := c.api.ListJobRequisitions(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("adp_wfn: list %q: %w", cid, err)
	}
	switch v := res.(type) {
	case *JobRequisitionList:
		out := &ListResult{
			Jobs:         v.JobRequisitions,
			TotalTrusted: !p.filtered(),
			HasMore:      len(v.JobRequisitions) >= PageSize,
		}
		out.TotalNumber = int(v.Meta.Value.TotalNumber.Value)
		return out, nil
	case *ListJobRequisitionsNotFound:
		return nil, fmt.Errorf("adp_wfn: list %q: %w", cid, ErrTenantNotFound)
	case *ListJobRequisitionsInternalServerError:
		return nil, fmt.Errorf("adp_wfn: list %q: upstream rejected the request", cid)
	default:
		return nil, fmt.Errorf("adp_wfn: list %q: unexpected response %T", cid, res)
	}
}

// Job returns one posting. jobID may be an itemID or an ExternalJobID; the
// tenant's own clientRequisitionID is not addressable and comes back as a
// record with no itemID, which is reported as [ErrJobNotFound] alongside a
// genuinely unknown id.
func (c *BoardClient) Job(ctx context.Context, cid, jobID, locale string) (*JobRequisition, error) {
	params := GetJobRequisitionParams{JobId: jobID, Cid: cid}
	if locale = strings.TrimSpace(locale); locale != "" {
		params.Locale = NewOptString(locale)
	}
	res, err := c.api.GetJobRequisition(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("adp_wfn: job %q for %q: %w", jobID, cid, err)
	}
	switch v := res.(type) {
	case *JobRequisition:
		if strings.TrimSpace(v.ItemID.Value) == "" {
			return nil, fmt.Errorf("adp_wfn: job %q for %q: %w", jobID, cid, ErrJobNotFound)
		}
		return v, nil
	case *GetJobRequisitionNotFound:
		return nil, fmt.Errorf("adp_wfn: job %q for %q: %w", jobID, cid, ErrTenantNotFound)
	default:
		return nil, fmt.Errorf("adp_wfn: job %q for %q: unexpected response %T", jobID, cid, res)
	}
}

// FilterCatalog is a tenant's published filter dimensions, already split by
// which column each one puts on the wire.
type FilterCatalog struct {
	// Locations are wire-ready `<value>,<qualifier>` pairs.
	Locations []FilterValue
	// WorkerCategories carry the oid to send and the label to show.
	WorkerCategories []FilterValue
}

// FilterValue pairs what a caller sees with what the upstream must be sent.
type FilterValue struct {
	Label string
	Wire  string
}

// SearchFilters returns tenant cid's published dimensions.
//
// An empty Locations does not mean location filtering is unavailable: at least
// one verified tenant publishes no location dimension yet filters correctly by
// city and state. It means only that there is no published list to validate a
// caller's input against.
func (c *BoardClient) SearchFilters(ctx context.Context, cid, locale string) (*FilterCatalog, error) {
	params := GetSearchFiltersParams{Cid: cid}
	if locale = strings.TrimSpace(locale); locale != "" {
		params.Locale = NewOptString(locale)
	}
	res, err := c.api.GetSearchFilters(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("adp_wfn: filters %q: %w", cid, err)
	}
	switch v := res.(type) {
	case *SearchFilters:
		out := &FilterCatalog{}
		for _, group := range v.Data {
			for _, info := range group.SearchFilterInfo {
				oid, value := strings.TrimSpace(info.Oid.Value), strings.TrimSpace(info.Value.Value)
				switch strings.ToUpper(strings.TrimSpace(group.FilterType.Value)) {
				case "FILTER LOCATION":
					// Locations put `value` on the wire and use `oid` as the
					// display label, and the two swap roles for job types.
					if value == "" {
						continue
					}
					out.Locations = append(out.Locations, FilterValue{Label: orFallback(oid, value), Wire: value})
				case "FILTER JOB TYPE":
					if oid == "" {
						continue
					}
					out.WorkerCategories = append(out.WorkerCategories, FilterValue{Label: orFallback(value, oid), Wire: oid})
				}
			}
		}
		return out, nil
	case *GetSearchFiltersNotFound:
		return nil, fmt.Errorf("adp_wfn: filters %q: %w", cid, ErrTenantNotFound)
	default:
		return nil, fmt.Errorf("adp_wfn: filters %q: unexpected response %T", cid, res)
	}
}

// TenantInfo is what the public API knows about who a tenant is.
type TenantInfo struct {
	// ClientName is ADP's own record of the client, passed through verbatim.
	// It is imperfect — observed truncation, and casing such as "Metlife" —
	// but it is read rather than reconstructed. Empty when unpublished.
	ClientName string
	// ClientID is a readable tenant slug, and the input
	// [BoardClient.ResolveLegacySlug] accepts.
	ClientID string
}

// TenantInfo returns tenant cid's identity. This is the only public place a
// company name appears: neither the job list nor the job detail carries one.
func (c *BoardClient) TenantInfo(ctx context.Context, cid string) (*TenantInfo, error) {
	res, err := c.api.GetClientFeatures(ctx, GetClientFeaturesParams{Cid: cid})
	if err != nil {
		return nil, fmt.Errorf("adp_wfn: tenant info %q: %w", cid, err)
	}
	switch v := res.(type) {
	case *MetaEnvelope:
		fields := metaStringFields(v)
		return &TenantInfo{
			ClientName: fields["ClientName"],
			ClientID:   fields["ClientID"],
		}, nil
	case *GetClientFeaturesNotFound:
		return nil, fmt.Errorf("adp_wfn: tenant info %q: %w", cid, ErrTenantNotFound)
	default:
		return nil, fmt.Errorf("adp_wfn: tenant info %q: unexpected response %T", cid, res)
	}
}

// PrimaryLocale returns the locale a tenant's postings live under.
//
// It reads the career-center content links, whose repeated Locale entries list
// the languages the UI is translated into, and takes the first — which matched
// the job-bearing locale on every tenant checked. The similarly named
// ClientDefaultLocale on client-features is deliberately not used: it reports
// en_US for a tenant whose postings exist only under en_CA.
//
// Returns "" when the tenant advertises none, leaving the caller to fall back
// to the upstream default rather than guess.
func (c *BoardClient) PrimaryLocale(ctx context.Context, cid string) (string, error) {
	res, err := c.api.GetCareerCenterContentLinks(ctx, GetCareerCenterContentLinksParams{Cid: cid})
	if err != nil {
		return "", fmt.Errorf("adp_wfn: locales %q: %w", cid, err)
	}
	switch v := res.(type) {
	case *MetaEnvelope:
		for _, f := range v.Meta.Value.CustomFieldGroup.Value.StringFields {
			if f.NameCode.Value.CodeValue.Value == "Locale" {
				if locale := strings.TrimSpace(f.StringValue.Value); locale != "" {
					return locale, nil
				}
			}
		}
		return "", nil
	case *GetCareerCenterContentLinksNotFound:
		return "", fmt.Errorf("adp_wfn: locales %q: %w", cid, ErrTenantNotFound)
	default:
		return "", fmt.Errorf("adp_wfn: locales %q: unexpected response %T", cid, res)
	}
}

// ResolveLegacySlug turns a readable client slug from a retired
// posting.html careers URL into a tenant GUID.
//
// The retired career center looks dead from the outside: without a cookie jar
// it redirects to itself until the client gives up. Given one, it completes a
// two-hop handshake and lands on a URL carrying the cid. Resolution is not
// universal — some slugs never leave posting.html — so a false return is
// ordinary rather than exceptional, and callers should treat the input as an
// unrecognized URL rather than an error.
//
// The jar is built per call and thrown away. Attaching one to the shared HTTP
// client would leak cookies into every other provider's requests.
func (c *BoardClient) ResolveLegacySlug(slug string) (string, bool) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", false
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", false
	}
	// Shallow-copy so the jar rides on this request only; the transport and
	// timeout stay shared with every other caller of c.hc.
	session := *c.hc
	session.Jar = jar

	// The interface this ultimately serves, ats.Adapter.ParseCareersURL, takes
	// no context, so cancellation cannot be inherited and a bound is set here
	// instead.
	ctx, cancel := context.WithTimeout(context.Background(), legacyResolveTimeout)
	defer cancel()

	target := c.legacyURL + "/jobs/apply/posting.html?client=" + url.QueryEscape(slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", false
	}
	rsp, err := session.Do(req)
	if err != nil {
		return "", false
	}
	defer rsp.Body.Close()

	cid := strings.TrimSpace(rsp.Request.URL.Query().Get("cid"))
	if cid == "" {
		return "", false
	}
	return cid, true
}

// ExternalJobID returns the id the public posting URL is built from. It is a
// different value from ItemID and lives in the custom-field bag; every row of
// every tenant swept carried one.
func ExternalJobID(j JobRequisition) string {
	for _, f := range j.CustomFieldGroup.Value.StringFields {
		if f.NameCode.Value.CodeValue.Value == "ExternalJobID" {
			return strings.TrimSpace(f.StringValue.Value)
		}
	}
	return ""
}

// PostedTime parses a row's post date, reporting false when the upstream sent
// none or sent something unparseable.
func PostedTime(j JobRequisition) (time.Time, bool) {
	raw := strings.TrimSpace(j.PostDate.Value)
	if raw == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// PrimaryLocation renders a row's first location for display, preferring the
// upstream's own composed label and falling back to the address parts.
func PrimaryLocation(j JobRequisition) string {
	if len(j.RequisitionLocations) == 0 {
		return ""
	}
	loc := j.RequisitionLocations[0]
	if short := strings.TrimSpace(loc.NameCode.Value.ShortName.Value); short != "" {
		return short
	}
	addr := loc.Address.Value
	parts := make([]string, 0, 2)
	if city := strings.TrimSpace(addr.CityName.Value); city != "" {
		parts = append(parts, city)
	}
	if state := strings.TrimSpace(addr.CountrySubdivisionLevel1.Value.CodeValue.Value); state != "" {
		parts = append(parts, state)
	}
	return strings.Join(parts, ", ")
}

// SalaryLine renders a row's published pay range, or "" when it has none.
// Roughly one row in eight carries this, and some tenants publish it on none.
func SalaryLine(j JobRequisition) string {
	pay, ok := j.PayGradeRange.Get()
	if !ok {
		return ""
	}
	minRate, minOK := pay.MinimumRate.Get()
	maxRate, maxOK := pay.MaximumRate.Get()
	if !minOK && !maxOK {
		return ""
	}
	currency := strings.TrimSpace(minRate.CurrencyCode.Value)
	if currency == "" {
		currency = strings.TrimSpace(maxRate.CurrencyCode.Value)
	}
	switch {
	case minOK && maxOK:
		return fmt.Sprintf("Pay range: %.2f - %.2f %s", minRate.AmountValue.Value, maxRate.AmountValue.Value, currency)
	case minOK:
		return fmt.Sprintf("Pay range: from %.2f %s", minRate.AmountValue.Value, currency)
	default:
		return fmt.Sprintf("Pay range: up to %.2f %s", maxRate.AmountValue.Value, currency)
	}
}

// JobURL renders the public posting page. jobID must be an ExternalJobID:
// the SPA's own share link uses that value, and an itemID produces a page
// that does not resolve to the posting.
func JobURL(cid, ccID, jobID, locale string) string {
	q := url.Values{}
	q.Set("cid", cid)
	if ccID != "" {
		q.Set("ccId", ccID)
	}
	if jobID != "" {
		q.Set("jobId", jobID)
	}
	if locale != "" {
		q.Set("lang", locale)
	}
	return "https://workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html?" + q.Encode()
}

// BoardURL renders a tenant's public career page.
func BoardURL(cid, ccID, locale string) string {
	return JobURL(cid, ccID, "", locale)
}

func metaStringFields(env *MetaEnvelope) map[string]string {
	out := map[string]string{}
	for _, f := range env.Meta.Value.CustomFieldGroup.Value.StringFields {
		key := strings.TrimSpace(f.NameCode.Value.CodeValue.Value)
		if key == "" {
			continue
		}
		if _, seen := out[key]; seen {
			continue
		}
		out[key] = strings.TrimSpace(f.StringValue.Value)
	}
	return out
}

func orFallback(preferred, fallback string) string {
	if preferred != "" {
		return preferred
	}
	return fallback
}
