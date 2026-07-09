package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	nvidia "github.com/amikai/openings-mcp/internal/provider/nvidia"
)

// nvidiaSiteURL is the public careers origin used to resolve ExternalPath.
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
	if err := ff.Parse(fs, os.Args[1:]); err != nil {
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

	search, err := client.SearchJobs(ctx, &nvidia.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         *limit,
		Offset:        *offset,
		SearchText:    *searchText,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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

		location, titleSlug, split := nvidia.SplitExternalPath(job.ExternalPath.Value)
		if !split {
			fmt.Fprintf(os.Stderr, "could not split externalPath %q\n", job.ExternalPath.Value)
			fmt.Println()
			continue
		}
		detail, err := client.GetJobDetail(ctx, nvidia.GetJobDetailParams{Location: location, TitleSlug: titleSlug})
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.ExternalPath.Value, err)
			// ExternalPath needs the site's career-site segment or the link 404s.
			fmt.Printf("URL: %s%s\n", nvidiaSiteURL, job.ExternalPath.Value)
			// LocationsText is the aggregate fallback when detail is unavailable.
			if job.LocationsText.Set {
				fmt.Printf("Location: %s\n", job.LocationsText.Value)
			}
			fmt.Println()
			continue
		}
		// Prefer the site's authoritative URL; ExternalPath omits the career-site
		// segment and would 404 if used alone.
		if detail.JobPostingInfo.ExternalUrl.Set {
			fmt.Printf("URL: %s\n", detail.JobPostingInfo.ExternalUrl.Value)
		}
		printLocations(detail.JobPostingInfo)
		description, err := html2text.FromString(detail.JobPostingInfo.JobDescription, html2text.Options{})
		if err != nil {
			description = detail.JobPostingInfo.JobDescription
		}
		if description != "" {
			fmt.Printf("Description:\n%s\n", description)
		}
		fmt.Println()
	}
}

// buildAppliedFacets resolves flag labels to Workday facet IDs. Empty labels
// leave their facet fields nil.
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

// labels returns sorted keys with an empty sentinel so ff leaves the enum
// unset instead of choosing the first real value.
func labels[V any](table map[string]V) []string {
	l := make([]string, 0, len(table)+1)
	l = append(l, "")
	for label := range table {
		l = append(l, label)
	}
	sort.Strings(l)
	return l
}

// usageWithChoices adds the valid values to flag help.
func usageWithChoices[V any](base string, table map[string]V) string {
	choices := labels(table)[1:] // drop the leading "" no-filter sentinel
	// " | " (not ", ") because some labels (e.g. site names like
	// "US, CA, Santa Clara") contain commas themselves.
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}

// printLocations prints the itemized locations from job detail rather than
// Workday's aggregate JobSummary.LocationsText.
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
