package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	nvidia "github.com/amikai/job-mcp/internal/provider/nvidia"
)

var tagRE = regexp.MustCompile(`<[^>]+>`)

// nvidiaSiteURL is the public careers site origin, distinct from --base-url
// (the wday/cxs API origin). ExternalPath values (e.g.
// "/job/US-CA-Remote/...") are relative to this, under /NVIDIAExternalCareerSite.
const nvidiaSiteURL = "https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite"

// main issues a single JobsRequest built entirely from flags, then fetches
// GetJobDetail for every job the search returned.
func main() {
	fs := ff.NewFlagSet("nvidia")
	var (
		baseURL      = fs.StringLong("base-url", "https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite", "NVIDIA Workday CXS base URL")
		timeout      = fs.DurationLong("timeout", 60*time.Second, "request timeout")
		searchText   = fs.StringLong("search-text", "", "free-text keyword search")
		limit        = fs.IntLong("limit", 20, "page size (server caps this at 20)")
		offset       = fs.IntLong("offset", 0, "zero-based result offset")
		jobCategory  = fs.StringEnumLong("job-category", usageWithChoices("Job Category", nvidia.JobCategoryIDs), labels(nvidia.JobCategoryIDs)...)
		jobType      = fs.StringEnumLong("job-type", usageWithChoices("Job Type", nvidia.JobTypeIDs), labels(nvidia.JobTypeIDs)...)
		timeType     = fs.StringEnumLong("time-type", usageWithChoices("Time Type", nvidia.TimeTypeIDs), labels(nvidia.TimeTypeIDs)...)
		locationType = fs.StringEnumLong("location-type", usageWithChoices("Location Type", nvidia.LocationTypeIDs), labels(nvidia.LocationTypeIDs)...)
		country      = fs.StringEnumLong("country", usageWithChoices("Country", nvidia.CountryIDs), labels(nvidia.CountryIDs)...)
		site         = fs.StringEnumLong("site", usageWithChoices("City-level site", nvidia.SiteIDs), labels(nvidia.SiteIDs)...)
	)
	if err := ff.Parse(fs, os.Args[1:], ff.WithEnvVarPrefix("NVIDIA")); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Flags(fs))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}

	appliedFacets := buildAppliedFacets(*jobCategory, *jobType, *timeType, *locationType, *country, *site)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := nvidia.NewClient(*baseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	searchRes, err := client.SearchJobs(ctx, &nvidia.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         *limit,
		Offset:        *offset,
		SearchText:    *searchText,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	search, ok := searchRes.(*nvidia.JobsResponse)
	if !ok {
		fmt.Fprintf(os.Stderr, "search returned %T\n", searchRes)
		os.Exit(1)
	}

	fmt.Printf("NVIDIA Jobs Report\n")
	fmt.Printf("Found %d jobs; showing %d\n\n", search.Total, len(search.JobPostings))

	for i, job := range search.JobPostings {
		fmt.Printf("%d. %s\n", i+1, job.Title.Value)
		if job.ExternalPath.Value == "" {
			fmt.Println("(no detail available for this listing)")
			fmt.Println()
			continue
		}
		if job.PostedOn.Set {
			fmt.Printf("Posted: %s\n", job.PostedOn.Value)
		}

		location, titleSlug, split := splitExternalPath(job.ExternalPath.Value)
		if !split {
			fmt.Fprintf(os.Stderr, "could not split externalPath %q\n", job.ExternalPath.Value)
			fmt.Println()
			continue
		}
		detailRes, err := client.GetJobDetail(ctx, nvidia.GetJobDetailParams{Location: location, TitleSlug: titleSlug})
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.ExternalPath.Value, err)
			// Fallback URL when the detail fetch (which carries the
			// authoritative externalUrl) failed. Must include the site's
			// "/NVIDIAExternalCareerSite" path segment or the link 404s.
			fmt.Printf("URL: %s%s\n", nvidiaSiteURL, job.ExternalPath.Value)
			// LocationsText ("N Locations") is Workday's aggregate string; it's
			// the best we have left if the detail fetch (which itemizes) failed.
			if job.LocationsText.Set {
				fmt.Printf("Location: %s\n", job.LocationsText.Value)
			}
			fmt.Println()
			continue
		}
		detail, ok := detailRes.(*nvidia.JobDetailResponse)
		if !ok {
			fmt.Fprintf(os.Stderr, "job detail %s returned %T\n", job.ExternalPath.Value, detailRes)
			fmt.Println()
			continue
		}
		// detail.JobPostingInfo.ExternalUrl is the site's own authoritative URL
		// (includes the "/NVIDIAExternalCareerSite" site path segment); hand-
		// constructing it from ExternalPath alone omits that segment and 404s.
		if detail.JobPostingInfo.ExternalUrl.Set {
			fmt.Printf("URL: %s\n", detail.JobPostingInfo.ExternalUrl.Value)
		}
		printLocations(detail.JobPostingInfo)
		if description := plainText(detail.JobPostingInfo.JobDescription); description != "" {
			fmt.Printf("Description:\n%s\n", description)
		}
		fmt.Println()
	}
}

// buildAppliedFacets resolves each flag's human label to a Workday facet id
// via the facets.go lookup tables. Labels are already validated against the
// flag's enum at parse time, so a lookup miss here can't happen for a
// non-empty label. An empty label (flag not set) leaves that facet field nil.
func buildAppliedFacets(jobCategory, jobType, timeType, locationType, country, site string) nvidia.AppliedFacets {
	var af nvidia.AppliedFacets
	if jobCategory != "" {
		af.JobFamilyGroup = []nvidia.AppliedFacetsJobFamilyGroupItem{nvidia.JobCategoryIDs[jobCategory]}
	}
	if jobType != "" {
		af.WorkerSubType = []nvidia.AppliedFacetsWorkerSubTypeItem{nvidia.JobTypeIDs[jobType]}
	}
	if timeType != "" {
		af.TimeType = []nvidia.AppliedFacetsTimeTypeItem{nvidia.TimeTypeIDs[timeType]}
	}
	if locationType != "" {
		af.LocationHierarchy2 = []nvidia.AppliedFacetsLocationHierarchy2Item{nvidia.LocationTypeIDs[locationType]}
	}
	if country != "" {
		af.LocationHierarchy1 = []nvidia.AppliedFacetsLocationHierarchy1Item{nvidia.CountryIDs[country]}
	}
	if site != "" {
		af.Locations = []nvidia.AppliedFacetsLocationsItem{nvidia.SiteIDs[site]}
	}
	return af
}

// labels returns the sorted keys of a facets.go lookup table, prefixed with
// "" so an ff.StringEnumLong flag can default to unset (no filter) instead
// of silently falling back to the first real label — ffval.Enum's zero
// Default only survives initialize() if it's itself in the Valid list.
func labels[V any](table map[string]V) []string {
	l := make([]string, 0, len(table)+1)
	l = append(l, "")
	for label := range table {
		l = append(l, label)
	}
	sort.Strings(l)
	return l
}

// usageWithChoices appends a comma-separated "one of: ..." list to base.
// ffhelp never introspects an ff.StringEnumLong's valid values on its own, so
// small enough choice sets are spelled out here to make -h self-documenting.
func usageWithChoices[V any](base string, table map[string]V) string {
	choices := labels(table)[1:] // drop the leading "" no-filter sentinel
	// " | " (not ", ") because some labels (e.g. site names like
	// "US, CA, Santa Clara") contain commas themselves.
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}

// splitExternalPath splits a JobSummary.ExternalPath (e.g.
// "/job/US-CA-Remote/Software-Engineer--CUDA-Q_JR2011649") into the two path
// segments GetJobDetail expects. The API rejects a single combined path
// parameter because standard URI encoders escape the "/" between them.
func splitExternalPath(externalPath string) (location, titleSlug string, ok bool) {
	trimmed := strings.TrimPrefix(externalPath, "/job/")
	location, titleSlug, ok = strings.Cut(trimmed, "/")
	return location, titleSlug, ok
}

// printLocations prints the itemized location(s) from a job detail response.
// Unlike JobSummary.LocationsText (which collapses multi-site postings into
// an aggregate string like "2 Locations"), JobPostingInfo carries the actual
// primary Location plus every AdditionalLocations entry.
func printLocations(info nvidia.JobPostingInfo) {
	locations := make([]string, 0, 1+len(info.AdditionalLocations))
	if info.Location.Set {
		locations = append(locations, info.Location.Value)
	}
	locations = append(locations, info.AdditionalLocations...)
	if len(locations) == 0 {
		return
	}
	if len(locations) == 1 {
		fmt.Printf("Location: %s\n", locations[0])
		return
	}
	fmt.Printf("Locations:\n")
	for _, l := range locations {
		fmt.Printf("  - %s\n", l)
	}
}

func plainText(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Fields(s)
	return strings.Join(lines, " ")
}
