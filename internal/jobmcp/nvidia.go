package jobmcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/amikai/job-mcp/internal/provider/nvidia"
	"github.com/jaytaylor/html2text"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var nvidiaSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword search across job title and description."
		},
		"job_category": {
			"type": "string",
			"description": "Job Category/Family Group filter.",
			"enum": [
				"Engineering", "Sales", "Operations", "Program Manager", "Marketing",
				"Research", "IT - Information Technology", "Univ Employment", "Finance",
				"Professional Services", "Human Resources", "Legal", "Facilities", "Business Development"
			]
		},
		"job_type": {
			"type": "string",
			"description": "Job Type / Worker Sub Type filter.",
			"enum": [
				"Regular Employee", "Management", "New College Graduate",
				"Intern (Fixed Term)", "Regular Employee (Fixed Term)", "Academic (Fixed Term)"
			]
		},
		"time_type": {
			"type": "string",
			"description": "Time Type filter.",
			"enum": ["Full time", "Part time"]
		},
		"location_type": {
			"type": "string",
			"description": "Location Type filter (Office vs Remote).",
			"enum": ["Office", "Remote"]
		},
		"country": {
			"type": "string",
			"description": "Country filter.",
			"enum": [
				"Armenia", "Australia", "Brazil", "Canada", "China", "Czechia", "Denmark",
				"Finland", "France", "Germany", "Greece", "Hong Kong", "Hungary", "India",
				"Israel", "Italy", "Japan", "Korea", "Mexico", "Netherlands", "Palestine",
				"Poland", "Romania", "Saudi Arabia", "Singapore", "Spain", "Sweden",
				"Switzerland", "Taiwan", "Thailand", "Ukraine", "United Arab Emirates",
				"United Kingdom", "United States", "Vietnam"
			]
		},
		"site": {
			"type": "string",
			"description": "City-level site filter.",
			"enum": [
				"Armenia, Yerevan", "Australia, Remote", "Australia, Sydney", "Brazil, Remote",
				"Brazil, Sao Paulo", "Canada, Remote", "Canada, Toronto", "China, Beijing",
				"China, Guangzhou", "China, Remote", "China, Shanghai", "China, Shenzhen",
				"Czechia, Remote", "Denmark, Roskilde", "Finland, Helsinki", "Finland, Remote",
				"France, Courbevoie", "France, Remote", "Germany, Berlin", "Germany, Munich",
				"Germany, Remote", "Germany, Wuerselen", "Greece, Athens", "Hong Kong, STP",
				"Hungary, Budapest", "Hungary, Remote", "India, Bengaluru", "India, Gurugram",
				"India, Hyderabad", "India, Mumbai", "India, Pune", "India, Remote",
				"Israel, Beer Sheva", "Israel, Raanana", "Israel, Tel Aviv", "Israel, Tel Hai",
				"Israel, Yokneam", "Italy, Remote", "Japan, Remote", "Japan, Tokyo",
				"Korea, Remote", "Korea, Seoul", "Mexico, Remote", "Netherlands, Amsterdam",
				"Netherlands, Remote", "Palestine, Rawabi", "Poland, Remote", "Poland, Warsaw",
				"Romania, Iasi", "Romania, Remote", "Saudi Arabia, Remote", "Singapore, Remote",
				"Singapore, Singapore-Suntec Tower", "Spain, Remote", "Sweden, Gothenburg",
				"Sweden, Lund", "Sweden, Remote", "Switzerland, Remote", "Switzerland, Zurich",
				"Taiwan, Hsinchu", "Taiwan, Remote", "Taiwan, Taipei", "Thailand, Remote",
				"UAE, Dubai", "UK, Belfast", "UK, Bristol", "UK, Cambridge", "UK, Reading",
				"UK, Remote", "Ukraine, Kyiv", "Ukraine, Remote", "US, AL, Madison",
				"US, AL, Remote", "US, AR, Remote", "US, AZ, Remote", "US, CA, Remote",
				"US, CA, San Jose", "US, CA, Santa Clara", "US, CO, Boulder", "US, CO, Remote",
				"US, CT, Remote", "US, DC, Remote", "US, DC, Washington", "US, FL, Remote",
				"US, GA, Remote", "US, IL, Champaign", "US, IL, Remote", "US, MA, Remote",
				"US, MA, Westford", "US, MD, Remote", "US, MI, Remote", "US, MN, Remote",
				"US, MO, St. Louis", "US, NC, Durham", "US, NC, Remote", "US, NH, Remote",
				"US, NJ, Remote", "US, NM, Remote", "US, NV, Remote", "US, NY, New York",
				"US, NY, Remote", "US, OH, Remote", "US, OR, Hillsboro", "US, OR, Remote",
				"US, PA, Remote", "US, Remote", "US, SC, Remote", "US, TN, Remote",
				"US, TX, Austin", "US, TX, Houston", "US, TX, Remote", "US, UT, Remote",
				"US, UT, Salt Lake City", "US, VA, Herndon", "US, VA, Remote", "US, WA, Redmond",
				"US, WA, Remote", "US, WA, Seattle", "Vietnam, Hanoi", "Vietnam, Ho Chi Minh City",
				"Vietnam, Remote"
			]
		},
		"limit": {
			"type": "integer",
			"description": "Page size. Server caps this at 20.",
			"minimum": 1,
			"maximum": 20
		},
		"offset": {
			"type": "integer",
			"description": "Zero-based result offset for pagination.",
			"minimum": 0
		}
	},
	"additionalProperties": false
}`)

var nvidiaSearchInputSchema = mustSchema(nvidiaSearchInputRawSchema)

type nvidiaSearchInput struct {
	Keyword      string `json:"keyword,omitempty"`
	JobCategory  string `json:"job_category,omitempty"`
	JobType      string `json:"job_type,omitempty"`
	TimeType     string `json:"time_type,omitempty"`
	LocationType string `json:"location_type,omitempty"`
	Country      string `json:"country,omitempty"`
	Site         string `json:"site,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	Offset       int    `json:"offset,omitempty"`
}

type nvidiaSearchOutput struct {
	Total int                `json:"total"`
	Data  []nvidiaJobSummary `json:"data"`
}

type nvidiaJobSummary struct {
	Title         string `json:"title"`
	ExternalPath  string `json:"external_path"`
	LocationsText string `json:"locations_text,omitempty"`
	PostedOn      string `json:"posted_on,omitempty"`
}

type nvidiaDetailInput struct {
	ExternalPath string `json:"external_path" jsonschema:"NVIDIA job external path (externalPath from search results, e.g. /job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916)."`
}

type nvidiaDetailOutput struct {
	Title               string   `json:"title"`
	Description         string   `json:"description" jsonschema:"Full job description as plain text/markdown."`
	Location            string   `json:"location,omitempty"`
	AdditionalLocations []string `json:"additional_locations,omitempty"`
	PostedOn            string   `json:"posted_on,omitempty"`
	TimeType            string   `json:"time_type,omitempty"`
	JobReqID            string   `json:"job_req_id,omitempty"`
	ExternalURL         string   `json:"external_url,omitempty"`
}

func buildNvidiaAppliedFacets(in *nvidiaSearchInput) (nvidia.AppliedFacets, error) {
	var af nvidia.AppliedFacets
	if in.JobCategory != "" {
		id, ok := nvidia.JobCategoryIDs[in.JobCategory]
		if !ok {
			return af, fmt.Errorf("invalid job_category %q", in.JobCategory)
		}
		af.JobFamilyGroup = []nvidia.AppliedFacetsJobFamilyGroupItem{id}
	}
	if in.JobType != "" {
		id, ok := nvidia.JobTypeIDs[in.JobType]
		if !ok {
			return af, fmt.Errorf("invalid job_type %q", in.JobType)
		}
		af.WorkerSubType = []nvidia.AppliedFacetsWorkerSubTypeItem{id}
	}
	if in.TimeType != "" {
		id, ok := nvidia.TimeTypeIDs[in.TimeType]
		if !ok {
			return af, fmt.Errorf("invalid time_type %q", in.TimeType)
		}
		af.TimeType = []nvidia.AppliedFacetsTimeTypeItem{id}
	}
	if in.LocationType != "" {
		id, ok := nvidia.LocationTypeIDs[in.LocationType]
		if !ok {
			return af, fmt.Errorf("invalid location_type %q", in.LocationType)
		}
		af.LocationHierarchy2 = []nvidia.AppliedFacetsLocationHierarchy2Item{id}
	}
	if in.Country != "" {
		id, ok := nvidia.CountryIDs[in.Country]
		if !ok {
			return af, fmt.Errorf("invalid country %q", in.Country)
		}
		af.LocationHierarchy1 = []nvidia.AppliedFacetsLocationHierarchy1Item{id}
	}
	if in.Site != "" {
		id, ok := nvidia.SiteIDs[in.Site]
		if !ok {
			return af, fmt.Errorf("invalid site %q", in.Site)
		}
		af.Locations = []nvidia.AppliedFacetsLocationsItem{id}
	}
	return af, nil
}

func splitNvidiaExternalPath(externalPath string) (location, titleSlug string, ok bool) {
	trimmed := strings.TrimPrefix(externalPath, "/job/")
	location, titleSlug, ok = strings.Cut(trimmed, "/")
	return location, titleSlug, ok
}

func nvidiaHTTPToMCPResponse(resp *nvidia.JobsResponse) *nvidiaSearchOutput {
	out := &nvidiaSearchOutput{
		Total: resp.Total,
		Data:  make([]nvidiaJobSummary, 0, len(resp.JobPostings)),
	}
	for _, j := range resp.JobPostings {
		out.Data = append(out.Data, nvidiaJobSummary{
			Title:         j.Title.Or(""),
			ExternalPath:  j.ExternalPath.Or(""),
			LocationsText: j.LocationsText.Or(""),
			PostedOn:      j.PostedOn.Or(""),
		})
	}
	return out
}

func nvidiaHTTPToMCPDetail(detail *nvidia.JobDetailResponse) *nvidiaDetailOutput {
	info := detail.JobPostingInfo
	descText, err := html2text.FromString(info.JobDescription, html2text.Options{})
	if err != nil {
		descText = info.JobDescription
	}
	return &nvidiaDetailOutput{
		Title:               info.Title,
		Description:         descText,
		Location:            info.Location.Or(""),
		AdditionalLocations: info.AdditionalLocations,
		PostedOn:            info.PostedOn.Or(""),
		TimeType:            info.TimeType.Or(""),
		JobReqID:            info.JobReqId.Or(""),
		ExternalURL:         info.ExternalUrl.Or(""),
	}
}

// RegisterNvidia registers the Nvidia search and job-detail tools.
func RegisterNvidia(s *mcp.Server, c *nvidia.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "nvidia_search_jobs",
		Description: "Search jobs on NVIDIA careers site by keyword and location, with optional job-category/job-type/time-type/location-type/country/site filters.",
		InputSchema: nvidiaSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *nvidiaSearchInput) (*mcp.CallToolResult, *nvidiaSearchOutput, error) {
		af, err := buildNvidiaAppliedFacets(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 20
		} else if limit > 20 {
			limit = 20
		}
		offset := in.Offset
		if offset < 0 {
			offset = 0
		}
		res, err := c.SearchJobs(ctx, &nvidia.JobsRequest{
			AppliedFacets: af,
			Limit:         limit,
			Offset:        offset,
			SearchText:    in.Keyword,
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, ok := res.(*nvidia.JobsResponse)
		if !ok {
			return errorResult(fmt.Errorf("job search returned %T", res)), nil, nil
		}
		return nil, nvidiaHTTPToMCPResponse(resp), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "nvidia_get_job_detail",
		Description: "Get the full job description and requirements for an NVIDIA job by external path.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *nvidiaDetailInput) (*mcp.CallToolResult, *nvidiaDetailOutput, error) {
		if in.ExternalPath == "" {
			return errorResult(fmt.Errorf("external_path is required")), nil, nil
		}
		location, titleSlug, ok := splitNvidiaExternalPath(in.ExternalPath)
		if !ok {
			return errorResult(fmt.Errorf("invalid external_path %q; must be in format '/job/{location}/{titleSlug}'", in.ExternalPath)), nil, nil
		}
		res, err := c.GetJobDetail(ctx, nvidia.GetJobDetailParams{
			Location:  location,
			TitleSlug: titleSlug,
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		detail, ok := res.(*nvidia.JobDetailResponse)
		if !ok {
			return errorResult(fmt.Errorf("job detail returned %T", res)), nil, nil
		}
		return nil, nvidiaHTTPToMCPDetail(detail), nil
	})
}
