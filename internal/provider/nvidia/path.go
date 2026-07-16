package nvidia

import "strings"

// SplitExternalPath splits [JobSummary.ExternalPath] (e.g.
// "/job/US-CA-Remote/Software-Engineer--CUDA-Q_JR2011649") into the two path
// segments [Client.GetJobDetail] expects. The API rejects a single combined path
// parameter because standard URI encoders escape the "/" between them.
// Inputs that don't match the exact /job/{location}/{title} shape return
// ok=false instead of producing unusable upstream requests.
func SplitExternalPath(externalPath string) (location, titleSlug string, ok bool) {
	rest, found := strings.CutPrefix(externalPath, "/job/")
	if !found {
		return "", "", false
	}
	location, titleSlug, ok = strings.Cut(rest, "/")
	missingSegs := !ok || location == "" || titleSlug == ""
	extraSlash := strings.Contains(titleSlug, "/")
	if missingSegs || extraSlash {
		return "", "", false
	}
	return location, titleSlug, true
}
