package main

import (
	"bytes"
	"testing"

	cake "github.com/amikai/openings-mcp/internal/provider/cake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSearchRequestUnfilteredByDefault(t *testing.T) {
	got, err := buildSearchRequest(searchFlags{keyword: "Golang", sort: "popularity", perPage: 10})
	require.NoError(t, err)

	want := cake.JobSearchRequest{
		Query:   "Golang",
		SortBy:  cake.JobSearchRequestSortByPopularity,
		PerPage: cake.NewOptInt(10),
	}
	assert.Equal(t, want, got)
}

func TestBuildSearchRequestResolvesAllFilters(t *testing.T) {
	got, err := buildSearchRequest(searchFlags{
		keyword:        "Golang",
		sort:           "latest",
		page:           2,
		perPage:        10,
		locations:      []string{"台灣"},
		professions:    []string{"it_back-end-engineer"},
		jobTypes:       []string{"full_time"},
		seniorities:    []string{"mid_senior_level"},
		years:          []string{"5_10"},
		managements:    []string{"none"},
		remotes:        []string{"partial_remote_work"},
		inclusivities:  []string{"lgbtq"},
		langs:          []string{"Chinese"},
		salaryType:     "per_month",
		salaryCurrency: "TWD",
		salaryMin:      60000,
		salaryMax:      150000,
		companySizes:   []string{"51_200"},
		sectors:        []string{"tech_software"},
		techLabels:     []string{"go"},
	})
	require.NoError(t, err)

	want := cake.JobSearchRequest{
		Query:   "Golang",
		SortBy:  cake.JobSearchRequestSortByLatest,
		Page:    cake.NewOptInt(2),
		PerPage: cake.NewOptInt(10),
		Filters: cake.JobSearchFilters{
			Locations:          []string{"台灣"},
			Professions:        []string{"it_back-end-engineer"},
			JobTypes:           []cake.JobSearchFiltersJobTypesItem{cake.JobSearchFiltersJobTypesItemFullTime},
			SeniorityLevels:    []cake.JobSearchFiltersSeniorityLevelsItem{cake.JobSearchFiltersSeniorityLevelsItemMidSeniorLevel},
			YearOfSeniority:    []cake.JobSearchFiltersYearOfSeniorityItem{cake.JobSearchFiltersYearOfSeniorityItem510},
			NumberOfManagement: []cake.JobSearchFiltersNumberOfManagementItem{cake.JobSearchFiltersNumberOfManagementItemNone},
			Remote:             []cake.JobSearchFiltersRemoteItem{cake.JobSearchFiltersRemoteItemPartialRemoteWork},
			InclusivityTraits:  []cake.JobSearchFiltersInclusivityTraitsItem{cake.JobSearchFiltersInclusivityTraitsItemLgbtq},
			LangNames:          []string{"Chinese"},
			Salary: cake.NewOptJobSearchFiltersSalary(cake.JobSearchFiltersSalary{
				Type:     cake.NewOptJobSearchFiltersSalaryType(cake.JobSearchFiltersSalaryTypePerMonth),
				Currency: cake.NewOptJobSearchFiltersSalaryCurrency(cake.JobSearchFiltersSalaryCurrencyTWD),
				Min:      cake.NewOptInt(60000),
				Max:      cake.NewOptInt(150000),
			}),
			Page: cake.NewOptJobSearchFiltersPage(cake.JobSearchFiltersPage{
				NumberOfEmployees: []cake.JobSearchFiltersPageNumberOfEmployeesItem{cake.JobSearchFiltersPageNumberOfEmployeesItem51200},
				Sectors:           []string{"tech_software"},
				TechLabels:        []string{"go"},
			}),
		},
	}
	assert.Equal(t, want, got)
}

func TestBuildSearchRequestUnknownJobType(t *testing.T) {
	_, err := buildSearchRequest(searchFlags{sort: "popularity", jobTypes: []string{"bogus"}})
	require.ErrorContains(t, err, "--job-type")
}

func TestBuildSearchRequestUnknownCompanySize(t *testing.T) {
	_, err := buildSearchRequest(searchFlags{sort: "popularity", companySizes: []string{"bogus"}})
	require.ErrorContains(t, err, "--company-size")
}

func TestBuildSearchRequestZeroPagesLeftUnset(t *testing.T) {
	got, err := buildSearchRequest(searchFlags{sort: "popularity"})
	require.NoError(t, err)
	assert.False(t, got.Page.Set)
	assert.False(t, got.PerPage.Set)
}

func TestWriteReportIncludesEveryJobDetail(t *testing.T) {
	search := &cake.JobSearchResponse{
		TotalEntries: cake.NewNilInt(2),
		TotalPages:   cake.NewNilInt(1),
		PerPage:      cake.NewNilInt(20),
		CurrentPage:  cake.NewNilInt(1),
		Data: []cake.JobSearchItem{
			{Path: "go-engineer", Title: cake.NewNilString("Go Engineer"), Description: cake.NewNilString("Go preview")},
			{Path: "backend-engineer", Title: cake.NewNilString("Backend Engineer"), Description: cake.NewNilString("Backend preview")},
		},
	}
	details := map[string]*cake.JobDetail{
		"go-engineer":      {Path: cake.NewNilString("go-engineer"), Title: cake.NewNilString("Go Engineer"), Description: cake.NewNilString("<p>Build Go services</p>"), Requirements: cake.NewNilString("<p>Go</p>")},
		"backend-engineer": {Path: cake.NewNilString("backend-engineer"), Title: cake.NewNilString("Backend Engineer"), Description: cake.NewNilString("<p>Build APIs</p>"), Requirements: cake.NewNilString("")},
	}

	var buf bytes.Buffer
	writeReport(&buf, "Golang", search, details)
	got := buf.String()

	for _, want := range []string{
		"Cake Jobs Report",
		"Keyword: Golang",
		"Found 2 jobs (page 1/1); showing 2",
		"[go-engineer] Go Engineer",
		"Build Go services",
		"Requirements: Go",
		"[backend-engineer] Backend Engineer",
		"Build APIs",
	} {
		assert.Contains(t, got, want)
	}
}
