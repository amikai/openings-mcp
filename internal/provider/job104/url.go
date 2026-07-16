package job104

import (
	"net/url"
	"strings"
)

// JobCodeFromURL extracts the job code from a 104 job posting URL's trailing
// path segment (e.g. https://www.104.com.tw/job/8zsac -> 8zsac). The search
// response's jobNo, by contrast, is 104's internal listing id and 404s if
// passed to [Client.GetJobDetail].
func JobCodeFromURL(raw string) string {
	path := raw
	if u, err := url.Parse(raw); err == nil {
		path = u.Path
	}
	path = strings.TrimRight(path, "/")
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
