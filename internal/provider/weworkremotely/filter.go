package weworkremotely

import "strings"

// FilterOptions narrows a jobs dump client-side. Zero-valued fields don't
// filter; set fields must all match (AND semantics).
type FilterOptions struct {
	// Keyword is a case-insensitive substring matched against a job's
	// title, skills, and HTML description.
	Keyword string
	// Category is a case-insensitive exact match against a [Categories]
	// display name (e.g. "Full-Stack Programming"). [Client.Search] uses
	// a recognized value to fetch only that feed; an unrecognized value
	// still filters (against Job.Category), it just costs the full dump.
	Category string
	// Company is a case-insensitive substring of the company name.
	Company string
	// Type is an exact, case-insensitive match against Job.Type (e.g.
	// "Full-Time", "Contract").
	Type string
	// Region is a case-insensitive substring matched against Region,
	// Country, and State combined — WWR splits location across all three
	// with no single reliable field.
	Region string
}

// FilterJobs returns the jobs matching every set field of opts, in their
// original order. The input slice is never modified.
func FilterJobs(jobs []Job, opts FilterOptions) []Job {
	keyword := strings.ToLower(opts.Keyword)
	category := strings.ToLower(opts.Category)
	company := strings.ToLower(opts.Company)
	jobType := strings.ToLower(opts.Type)
	region := strings.ToLower(opts.Region)

	var out []Job
	for _, j := range jobs {
		if keyword != "" {
			haystack := strings.ToLower(j.Title + " " + j.Skills + " " + j.Description)
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		if category != "" && strings.ToLower(j.Category) != category {
			continue
		}
		if company != "" && !strings.Contains(strings.ToLower(j.Company), company) {
			continue
		}
		if jobType != "" && strings.ToLower(j.Type) != jobType {
			continue
		}
		if region != "" {
			haystack := strings.ToLower(j.Region + " " + j.Country + " " + j.State)
			if !strings.Contains(haystack, region) {
				continue
			}
		}
		out = append(out, j)
	}
	return out
}
