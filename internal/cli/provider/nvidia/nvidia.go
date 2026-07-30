// Package nvidia implements the "openings-mcp nvidia" debug CLI, for manual
// checks against the live surface that internal/provider/nvidia documents.
package nvidia

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	nvidiaprovider "github.com/amikai/openings-mcp/internal/provider/nvidia"
)

type options struct {
	baseURL      string
	timeout      time.Duration
	searchText   string
	limit        int
	offset       int
	jobCategory  string
	jobType      string
	timeType     string
	locationType string
	country      string
	site         string
}

// NewCommand returns a cobra.Command for nvidia.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "nvidia",
		Short:        "Search NVIDIA Workday jobs and view position details",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.jobCategory != "" {
				if _, ok := nvidiaprovider.JobCategoryIDs[opts.jobCategory]; !ok {
					return fmt.Errorf("invalid job-category %q", opts.jobCategory)
				}
			}
			if opts.jobType != "" {
				if _, ok := nvidiaprovider.JobTypeIDs[opts.jobType]; !ok {
					return fmt.Errorf("invalid job-type %q", opts.jobType)
				}
			}
			if opts.timeType != "" {
				if _, ok := nvidiaprovider.TimeTypeIDs[opts.timeType]; !ok {
					return fmt.Errorf("invalid time-type %q", opts.timeType)
				}
			}
			if opts.locationType != "" {
				if _, ok := nvidiaprovider.LocationTypeIDs[opts.locationType]; !ok {
					return fmt.Errorf("invalid location-type %q", opts.locationType)
				}
			}
			if opts.country != "" {
				if _, ok := nvidiaprovider.CountryIDs[opts.country]; !ok {
					return fmt.Errorf("invalid country %q", opts.country)
				}
			}
			if opts.site != "" {
				if _, ok := nvidiaprovider.SiteIDs[opts.site]; !ok {
					return fmt.Errorf("invalid site %q", opts.site)
				}
			}

			appliedFacets := buildAppliedFacets(facetFlags{
				jobCategory:  opts.jobCategory,
				jobType:      opts.jobType,
				timeType:     opts.timeType,
				locationType: opts.locationType,
				country:      opts.country,
				site:         opts.site,
			})

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			client, err := nvidiaprovider.NewClient(opts.baseURL)
			if err != nil {
				return err
			}

			search, err := client.SearchJobs(ctx, &nvidiaprovider.JobsRequest{
				AppliedFacets: appliedFacets,
				Limit:         opts.limit,
				Offset:        opts.offset,
				SearchText:    opts.searchText,
			})
			if err != nil {
				return err
			}

			fmt.Printf("NVIDIA Jobs Report\n")
			fmt.Printf("Found %d jobs; showing %d\n\n", search.Total.Value, len(search.JobPostings))

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

				location, titleSlug, split := nvidiaprovider.SplitExternalPath(job.ExternalPath.Value)
				if !split {
					fmt.Fprintf(os.Stderr, "could not split externalPath %q\n", job.ExternalPath.Value)
					fmt.Println()
					continue
				}
				detail, err := client.GetJobDetail(ctx, nvidiaprovider.GetJobDetailParams{Location: location, TitleSlug: titleSlug})
				if err != nil {
					fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.ExternalPath.Value, err)
					fmt.Printf("URL: %s%s\n", nvidiaprovider.DefaultSiteURL, job.ExternalPath.Value)
					if job.LocationsText.Set {
						fmt.Printf("Location: %s\n", job.LocationsText.Value)
					}
					fmt.Println()
					continue
				}
				if detail.JobPostingInfo.ExternalUrl.Set {
					fmt.Printf("URL: %s\n", detail.JobPostingInfo.ExternalUrl.Value)
				}
				printLocations(detail.JobPostingInfo)
				description, err := html2text.FromString(detail.JobPostingInfo.JobDescription.Value, html2text.Options{})
				if err != nil {
					description = detail.JobPostingInfo.JobDescription.Value
				}
				if description != "" {
					fmt.Printf("Description:\n%s\n", description)
				}
				fmt.Println()
			}
			return nil
		},
	}

	rootCmd.Flags().StringVar(&opts.baseURL, "base-url", nvidiaprovider.DefaultBaseURL, "NVIDIA Workday CXS base URL")
	rootCmd.Flags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.Flags().StringVar(&opts.searchText, "search-text", "", "free-text keyword search")
	rootCmd.Flags().IntVar(&opts.limit, "limit", 20, "page size (server caps this at 20)")
	rootCmd.Flags().IntVar(&opts.offset, "offset", 0, "zero-based result offset")
	rootCmd.Flags().StringVar(&opts.jobCategory, "job-category", "", usageWithChoices("Job Category", nvidiaprovider.JobCategoryIDs))
	rootCmd.Flags().StringVar(&opts.jobType, "job-type", "", usageWithChoices("Job Type", nvidiaprovider.JobTypeIDs))
	rootCmd.Flags().StringVar(&opts.timeType, "time-type", "", usageWithChoices("Time Type", nvidiaprovider.TimeTypeIDs))
	rootCmd.Flags().StringVar(&opts.locationType, "location-type", "", usageWithChoices("Location Type", nvidiaprovider.LocationTypeIDs))
	rootCmd.Flags().StringVar(&opts.country, "country", "", usageWithChoices("Country", nvidiaprovider.CountryIDs))
	rootCmd.Flags().StringVar(&opts.site, "site", "", usageWithChoices("City-level site", nvidiaprovider.SiteIDs))

	return rootCmd
}

// facetFlags carries the parsed flag values into buildAppliedFacets.
type facetFlags struct {
	jobCategory  string
	jobType      string
	timeType     string
	locationType string
	country      string
	site         string
}

// buildAppliedFacets resolves each flag's human label to a Workday facet id
// via the facets.go lookup tables. Labels are already validated against the
// flag's enum at parse time, so a lookup miss here can't happen for a
// non-empty label. An empty label (flag not set) leaves that facet field nil.
func buildAppliedFacets(f facetFlags) nvidiaprovider.AppliedFacets {
	var af nvidiaprovider.AppliedFacets
	if f.jobCategory != "" {
		af.JobFamilyGroup = []nvidiaprovider.AppliedFacetsJobFamilyGroupItem{nvidiaprovider.JobCategoryIDs[f.jobCategory]}
	}
	if f.jobType != "" {
		af.WorkerSubType = []nvidiaprovider.AppliedFacetsWorkerSubTypeItem{nvidiaprovider.JobTypeIDs[f.jobType]}
	}
	if f.timeType != "" {
		af.TimeType = []nvidiaprovider.AppliedFacetsTimeTypeItem{nvidiaprovider.TimeTypeIDs[f.timeType]}
	}
	if f.locationType != "" {
		af.LocationHierarchy2 = []nvidiaprovider.AppliedFacetsLocationHierarchy2Item{nvidiaprovider.LocationTypeIDs[f.locationType]}
	}
	if f.country != "" {
		af.LocationHierarchy1 = []nvidiaprovider.AppliedFacetsLocationHierarchy1Item{nvidiaprovider.CountryIDs[f.country]}
	}
	if f.site != "" {
		af.Locations = []nvidiaprovider.AppliedFacetsLocationsItem{nvidiaprovider.SiteIDs[f.site]}
	}
	return af
}

// labels returns the sorted keys of a facets.go lookup table, prefixed with
// "" so an ff.StringEnumLong flag can default to unset (no filter) instead
// of silently falling back to the first real label — ffval.Enum's zero
// Default only survives initialize() if it's itself in the Valid list.
func labels[V any](table map[string]V) []string {
	return append([]string{""}, slices.Sorted(maps.Keys(table))...)
}

// usageWithChoices appends a comma-separated "one of: ..." list to base.
// ffhelp never introspects an ff.StringEnumLong's valid values on its own, so
// small enough choice sets are spelled out here to make -h self-documenting.
func usageWithChoices[V any](base string, table map[string]V) string {
	choices := labels(table)[1:]
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}

// printLocations prints the itemized location(s) from a job detail response.
// Unlike JobSummary.LocationsText (which collapses multi-site postings into
// an aggregate string like "2 Locations"), JobPostingInfo carries the actual
// primary Location plus every AdditionalLocations entry.
func printLocations(info nvidiaprovider.JobPostingInfo) {
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
