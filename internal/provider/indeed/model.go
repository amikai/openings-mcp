package indeed

// Job type filter keys (the "attributes" composite filter's keyword keys),
// ported from python-jobspy's job_type_key_mapping.
const (
	JobTypeFullTime   = "CF3CP"
	JobTypePartTime   = "75GKK"
	JobTypeContract   = "NJXCK"
	JobTypeInternship = "VDTG7"
)

// remoteAttributeKey is the composite filter's keyword key for "Remote"
// (jobspy's is_remote branch), applied alongside JobType* when Remote is set.
const remoteAttributeKey = "DSQF7"

// JobTypeIDs maps a human label to its JobsRequest.JobType value.
var JobTypeIDs = map[string]string{
	"Full-time":  JobTypeFullTime,
	"Part-time":  JobTypePartTime,
	"Contract":   JobTypeContract,
	"Internship": JobTypeInternship,
}

// JobsRequest is a search query against Indeed's jobSearch GraphQL field.
type JobsRequest struct {
	Keywords string
	// Location is free-text, e.g. "Taipei". Must correspond to Country —
	// see API.md's Key Behaviors on what a mismatch does.
	Location string
	// RadiusMiles is the search radius around Location. nil defaults to 25
	// (python-jobspy's default); 0 is exact-location (Indeed accepts it).
	// A plain int cannot distinguish "omitted" from "zero".
	RadiusMiles *int
	// Country selects the indeed-co catalogue and the job_url domain via
	// CountryByName; empty defaults to DefaultCountryName for search only.
	// JobDetail requires an explicit country — jobData is country-scoped.
	Country string
	// Cursor pages through results: pass the previous JobsResponse's
	// NextCursor. Empty starts from the first page.
	Cursor string
	// Limit caps results per call, max 100 (the reference implementation's
	// jobs_per_page); defaults to 25 when 0.
	Limit int
	// HoursOld, JobType/Remote, and EasyApply are mutually exclusive filters
	// in the reference query shape (see API.md); when more than one is
	// set, HoursOld wins, then EasyApply, then JobType/Remote — the same
	// precedence as python-jobspy's _build_filters.
	HoursOld  int
	JobType   string // one of JobType* above
	Remote    bool
	EasyApply bool
}

// Compensation mirrors Indeed's compensation.{baseSalary,estimated} shape;
// nil when the posting doesn't disclose one (the common case). Amounts are
// float64 because Indeed returns fractional hourly ranges (e.g. 22.5–27.5).
type Compensation struct {
	MinAmount float64
	MaxAmount float64
	Currency  string
	// Interval is the raw unitOfWork value (YEAR, MONTH, WEEK, DAY, HOUR),
	// uppercased; left as Indeed sends it rather than remapped, since
	// callers needing python-jobspy's YEARLY/MONTHLY/... labels can map it
	// themselves.
	Interval string
}

// Job is a jobSearch result: a lean summary, no full description.
type Job struct {
	Key        string // Indeed's opaque job key; pass to Client.JobDetail.
	Title      string
	Company    string
	CompanyURL string
	Location   string
	// Country is the country name used for the search that produced this
	// row (DefaultCountryName when the caller omitted it). Pass it back
	// to JobDetail — jobData is country-scoped and an omitted/wrong
	// country yields a false empty result.
	Country string
	// JobURL is the Indeed-hosted posting page, built from the search
	// request's Country domain.
	JobURL string
	// PostedDate is YYYY-MM-DD, derived from datePublished (epoch millis).
	PostedDate string
	// JobTypes are Indeed's own attribute labels (e.g. "Full-time",
	// "Permanent"), passed through as-is rather than filtered to a fixed
	// enum.
	JobTypes     []string
	Compensation *Compensation
}

// JobsResponse is one page of jobSearch results.
type JobsResponse struct {
	Jobs []Job
	// NextCursor feeds JobsRequest.Cursor for the next page; empty means no
	// more results.
	NextCursor string
}

// Location is jobData's structured location, used only by JobDetail —
// Job (search) stays a flat formatted string by design (see its doc
// comment): a lean summary for up to 100 rows shouldn't carry per-row
// structured fields it never asked for.
type Location struct {
	Country     string // countryName, e.g. "台灣"
	CountryCode string // e.g. "TW"
	State       string // admin1Code, Indeed's own region code, e.g. "TPE"
	City        string
	// PostalCode and StreetAddress are usually empty: Indeed rarely
	// discloses either.
	PostalCode    string
	StreetAddress string
	// Formatted is Indeed's own human-readable rendering (formatted.long).
	Formatted string
}

// JobDetail is a jobData result: every field the query requests, not just
// a subset. jobData is already an Indeed-specific type — unlike
// python-jobspy's cross-site JobPost, nothing here needs to fit a shared
// schema, so there's no reason to leave data on the table.
type JobDetail struct {
	Key          string
	Title        string
	Company      string
	CompanyURL   string
	Location     Location
	JobURL       string
	PostedDate   string
	Description  string // HTML, as Indeed sends it.
	JobTypes     []string
	Compensation *Compensation

	// Source is the listing's source name: usually the employer's own name
	// for a direct-post job, but a third-party board's name when Indeed
	// aggregated the posting from elsewhere.
	Source string
	// DateIndexed is dateOnIndeed, YYYY-MM-DD: when Indeed indexed/last
	// refreshed this posting, distinct from PostedDate (datePublished, the
	// employer's original post date) — DateIndexed can be later for
	// reposted or refreshed listings.
	DateIndexed string

	CompanyWebsite     string
	CompanyIndustry    string
	CompanyEmployees   string
	CompanyRevenue     string
	CompanyDescription string
	CompanyLogo        string
	// CompanyAddresses lists the employer's disclosed office addresses,
	// when available (rare).
	CompanyAddresses []string
	// CompanyCEO and CompanyCEOPhoto come from the employer's dossier, when
	// Indeed has one on file.
	CompanyCEO      string
	CompanyCEOPhoto string
	// CompanyBannerImage is the employer's profile banner (headerImageUrl),
	// distinct from CompanyLogo (the square logo).
	CompanyBannerImage string

	// ApplyURL is recruit.viewJobUrl: the external ATS URL a poster
	// configured for direct apply, if any. Empty for Indeed-native apply
	// flows.
	ApplyURL string
	// DetailedSalary and WorkSchedule are free-text fields a poster can set
	// beyond the structured Compensation (e.g. a shift pattern); both are
	// usually empty.
	DetailedSalary string
	WorkSchedule   string
}
