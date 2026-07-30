package amazon

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/amikai/openings-mcp/internal/provider/amazon"
)

func TestSearchOptionsRequest(t *testing.T) {
	options := searchOptions{
		keyword:            "software engineer",
		countries:          []string{"TWN"},
		cities:             []string{"Taipei City"},
		jobCategories:      []string{"Software Development"},
		businessCategories: []string{"alexa-and-amazon-devices"},
		scheduleTypes:      []string{"Full-Time"},
		sort:               "recent",
		offset:             20,
		limit:              10,
	}
	assert.Equal(t, amazon.SearchRequest{
		Query:              "software engineer",
		Countries:          []string{"TWN"},
		Cities:             []string{"Taipei City"},
		JobCategories:      []string{"Software Development"},
		BusinessCategories: []string{"alexa-and-amazon-devices"},
		ScheduleTypes:      []string{"Full-Time"},
		Sort:               amazon.SearchJobsSortRecent,
		Offset:             20,
		Limit:              10,
	}, options.request())
}

func TestSummarize(t *testing.T) {
	applyURL, err := url.Parse("https://account.amazon.com/jobs/3164253/apply")
	assert.NoError(t, err)
	job := amazon.Job{
		IDIcims:            "3164253",
		JobPath:            "/en/jobs/3164253/software-dev-engineer-eero",
		Title:              "Software Dev Engineer, eero",
		Location:           "TW, TPE, Taipei",
		NormalizedLocation: "Taipei City, TWN",
		CountryCode:        "TWN",
		CompanyName:        "Amazon Development Center Taiwan Limited",
		JobCategory:        "Software Development",
		BusinessCategory:   "studentprograms",
		JobScheduleType:    "full-time",
		PostedDate:         "January 22, 2026",
		UpdatedTime:        "9 days",
		DescriptionShort:   "Build wireless products.",
		Description:        "Build<br/>wireless products.",
		URLNextStep:        *applyURL,
	}

	summary := summarize(job)
	assert.Equal(t, "3164253", summary.ID)
	assert.Equal(t, "https://www.amazon.jobs/en/jobs/3164253/software-dev-engineer-eero", summary.URL)
	assert.Equal(t, "Build wireless products.", summary.Description)

	detail := toDetail(job)
	assert.Equal(t, "https://account.amazon.com/jobs/3164253/apply", detail.ApplyURL)
	assert.Contains(t, detail.Description, "Build")
	assert.NotContains(t, detail.Description, "<br")
}

func TestRenderHTML(t *testing.T) {
	assert.Equal(t, "one\ntwo", renderHTML("one<br/>two"))
}
