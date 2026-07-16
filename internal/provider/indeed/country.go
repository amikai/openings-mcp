package indeed

import "strings"

// Country carries the two values Indeed derives from a country selection:
// Domain is the subdomain of the country-specific Indeed website (used to
// build public job_url links, e.g. "tw" -> tw.indeed.com), and APICode is
// the value the indeed-co request header must carry for that country's
// catalogue. The two must travel together — see API.md's Key
// Behaviors on what happens when indeed-co doesn't match the search
// location.
type Country struct {
	Domain  string
	APICode string
}

// DefaultCountryName is used when a caller doesn't specify one.
const DefaultCountryName = "taiwan"

// countries is ported from python-jobspy's Country enum (jobspy/model.py),
// trimmed to the two fields this client needs. Domain is NOT the country's
// full name: it's Indeed's own (usually two-letter) subdomain code, e.g.
// Taiwan's is "tw" (tw.indeed.com), Malaysia's is "malaysia"
// (malaysia.indeed.com), and the US's is "www" (www.indeed.com) — each
// taken verbatim from jobspy's Country.indeed_domain_value.
// Only countries that map to a real Indeed site. Entries accepted by
// python-jobspy but rejected live with BAD_USER_INPUT ("does not correspond
// to a valid Indeed site") are omitted: Bangladesh, Bulgaria, Croatia,
// Cyprus, Estonia, Latvia, Lithuania, Malta, Slovakia, Slovenia.
var countries = map[string]Country{
	"argentina":            {"ar", "AR"},
	"australia":            {"au", "AU"},
	"austria":              {"at", "AT"},
	"bahrain":              {"bh", "BH"},
	"belgium":              {"be", "BE"},
	"brazil":               {"br", "BR"},
	"canada":               {"ca", "CA"},
	"chile":                {"cl", "CL"},
	"china":                {"cn", "CN"},
	"colombia":             {"co", "CO"},
	"costa rica":           {"cr", "CR"},
	"czech republic":       {"cz", "CZ"},
	"czechia":              {"cz", "CZ"},
	"denmark":              {"dk", "DK"},
	"ecuador":              {"ec", "EC"},
	"egypt":                {"eg", "EG"},
	"finland":              {"fi", "FI"},
	"france":               {"fr", "FR"},
	"germany":              {"de", "DE"},
	"greece":               {"gr", "GR"},
	"hong kong":            {"hk", "HK"},
	"hungary":              {"hu", "HU"},
	"india":                {"in", "IN"},
	"indonesia":            {"id", "ID"},
	"ireland":              {"ie", "IE"},
	"israel":               {"il", "IL"},
	"italy":                {"it", "IT"},
	"japan":                {"jp", "JP"},
	"kuwait":               {"kw", "KW"},
	"luxembourg":           {"lu", "LU"},
	"malaysia":             {"malaysia", "MY"},
	"mexico":               {"mx", "MX"},
	"morocco":              {"ma", "MA"},
	"netherlands":          {"nl", "NL"},
	"new zealand":          {"nz", "NZ"},
	"nigeria":              {"ng", "NG"},
	"norway":               {"no", "NO"},
	"oman":                 {"om", "OM"},
	"pakistan":             {"pk", "PK"},
	"panama":               {"pa", "PA"},
	"peru":                 {"pe", "PE"},
	"philippines":          {"ph", "PH"},
	"poland":               {"pl", "PL"},
	"portugal":             {"pt", "PT"},
	"qatar":                {"qa", "QA"},
	"romania":              {"ro", "RO"},
	"saudi arabia":         {"sa", "SA"},
	"singapore":            {"sg", "SG"},
	"south africa":         {"za", "ZA"},
	"south korea":          {"kr", "KR"},
	"spain":                {"es", "ES"},
	"sweden":               {"se", "SE"},
	"switzerland":          {"ch", "CH"},
	"taiwan":               {"tw", "TW"},
	"thailand":             {"th", "TH"},
	"turkey":               {"tr", "TR"},
	"türkiye":              {"tr", "TR"},
	"ukraine":              {"ua", "UA"},
	"united arab emirates": {"ae", "AE"},
	"uk":                   {"uk", "GB"},
	"united kingdom":       {"uk", "GB"},
	"usa":                  {"www", "US"},
	"us":                   {"www", "US"},
	"united states":        {"www", "US"},
	"uruguay":              {"uy", "UY"},
	"venezuela":            {"ve", "VE"},
	"vietnam":              {"vn", "VN"},
}

// CountryByName looks up a Country by its common English name, matching the
// aliases python-jobspy's Country enum accepts (e.g. "US", "USA", and
// "United States" all resolve to the same entry). Matching is
// case-insensitive; an unknown name reports ok=false.
func CountryByName(name string) (Country, bool) {
	c, ok := countries[strings.ToLower(strings.TrimSpace(name))]
	return c, ok
}
