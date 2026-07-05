package workday

import (
	"fmt"
	"net/url"
	"strings"
)

// SplitExternalPath splits a JobSummary.ExternalPath (e.g.
// "/job/US-CA-Remote/Software-Engineer--CUDA_JR12345") into the two path
// segments GetJobDetail expects. The API rejects a single combined path
// parameter because standard URI encoders escape the "/" between them.
func SplitExternalPath(externalPath string) (location, titleSlug string, ok bool) {
	location, titleSlug, ok = strings.Cut(strings.TrimPrefix(externalPath, "/job/"), "/")
	return location, titleSlug, ok
}

// PublicSiteURL derives a Workday tenant's public-facing (non-API) career
// site origin from its CXS base URL, by taking the base URL's last path
// segment — the "{site}" segment shared by both URL shapes (see
// openapi.yaml's "Multi-tenant URL shape" note). For example:
//
//	https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite
//	  -> https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite
//
// Confirmed against NVIDIA's tenant; not verified against any other.
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
