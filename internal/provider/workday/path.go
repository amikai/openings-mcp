package workday

import (
	"fmt"
	"net/url"
	"strings"
)

// JobDetailKeyFromPath extracts the two GetJobDetail path parameters out of
// a JobSummary.ExternalPath (e.g.
// "/job/US-CA-Remote/Software-Engineer--CUDA_JR12345"). The API rejects a
// single combined path parameter because standard URI encoders escape the
// "/" between them, so callers need the segments split apart.
//
// It only accepts the exact "/job/{location}/{titleSlug}" shape, and
// returns ok=false for anything else: a missing "/job/" prefix, an empty
// segment, or extra path segments, whose "/" a URI encoder would
// percent-encode into a shape the server rejects. Callers can then fall
// back instead of sending a request that's guaranteed to fail.
func JobDetailKeyFromPath(externalPath string) (location, titleSlug string, ok bool) {
	rest, found := strings.CutPrefix(externalPath, "/job/")
	if !found {
		return "", "", false
	}
	location, titleSlug, ok = strings.Cut(rest, "/")
	if !ok || location == "" || titleSlug == "" || strings.Contains(titleSlug, "/") {
		return "", "", false
	}
	return location, titleSlug, true
}

// PublicSiteURL derives a Workday tenant's public-facing (non-API) career
// site origin from its CXS base URL. It takes the base URL's last path
// segment, the "{site}" segment shared by both URL shapes (see
// openapi.yaml's "Multi-tenant URL shape" note). For example:
//
//	https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite
//	  -> https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite
func PublicSiteURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL %q: %w", baseURL, err)
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	site := segments[len(segments)-1]
	if site == "" {
		return "", fmt.Errorf("base URL %q has no path segment to derive a site from", baseURL)
	}
	return fmt.Sprintf("%s://%s/%s", u.Scheme, u.Host, site), nil
}
