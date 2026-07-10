package openingsmcp

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"github.com/amikai/openings-mcp/internal/provider/nvidia"
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
			"description": "Job category filter.",
			"enum": [
				"Engineering", "Sales", "Operations", "Program Manager", "Marketing",
				"Research", "IT - Information Technology", "Univ Employment", "Finance",
				"Professional Services", "Human Resources", "Legal", "Facilities", "Business Development"
			]
		},
		"job_type": {
			"type": "string",
			"description": "Job type filter.",
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
	"required": ["keyword", "country"],
	"additionalProperties": false
}`)

var nvidiaSearchInputSchema = mustSchema(nvidiaSearchInputRawSchema)

type nvidiaSearchInput struct {
	Keyword      string `json:"keyword"` // required
	Country      string `json:"country"` // required
	JobCategory  string `json:"job_category,omitempty"`
	JobType      string `json:"job_type,omitempty"`
	TimeType     string `json:"time_type,omitempty"`
	LocationType string `json:"location_type,omitempty"`
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
	LocationsText string `json:"locations_text,omitempty" jsonschema:"Human-readable location(s); may be an aggregate count like '3 Locations' instead of a specific site when the job posts to multiple locations."`
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

func nvidiaMCPToHTTPRequest(in *nvidiaSearchInput) (*nvidia.JobsRequest, error) {
	var req nvidia.JobsRequest
	// The schema already marks keyword and country required; this guards
	// direct callers and clients that skip schema validation — a missing
	// country fails its enum Validate below (empty label maps to the zero
	// value).
	if in.Keyword == "" {
		return nil, errors.New("keyword is required")
	}
	req.SearchText = in.Keyword

	country := nvidia.CountryIDs[in.Country]
	if err := country.Validate(); err != nil {
		return nil, fmt.Errorf("invalid country %q: %w", in.Country, err)
	}
	req.AppliedFacets.LocationHierarchy1 = []nvidia.AppliedFacetsLocationHierarchy1Item{country}

	if in.JobCategory != "" {
		id := nvidia.JobCategoryIDs[in.JobCategory]
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf("invalid job_category %q: %w", in.JobCategory, err)
		}
		req.AppliedFacets.JobFamilyGroup = []nvidia.AppliedFacetsJobFamilyGroupItem{id}
	}

	if in.JobType != "" {
		id := nvidia.JobTypeIDs[in.JobType]
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf("invalid job_type %q: %w", in.JobType, err)
		}
		req.AppliedFacets.WorkerSubType = []nvidia.AppliedFacetsWorkerSubTypeItem{id}
	}

	if in.TimeType != "" {
		id := nvidia.TimeTypeIDs[in.TimeType]
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf("invalid time_type %q: %w", in.TimeType, err)
		}
		req.AppliedFacets.TimeType = []nvidia.AppliedFacetsTimeTypeItem{id}
	}

	if in.LocationType != "" {
		id := nvidia.LocationTypeIDs[in.LocationType]
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf("invalid location_type %q: %w", in.LocationType, err)
		}
		req.AppliedFacets.LocationHierarchy2 = []nvidia.AppliedFacetsLocationHierarchy2Item{id}
	}

	if in.Site != "" {
		id := nvidia.SiteIDs[in.Site]
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf("invalid site %q: %w", in.Site, err)
		}
		req.AppliedFacets.Locations = []nvidia.AppliedFacetsLocationsItem{id}
	}

	req.Limit = cmp.Or(in.Limit, 20)
	req.Offset = in.Offset
	return &req, nil
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
		Description: "Search jobs on the NVIDIA careers site.",
		Annotations: &mcp.ToolAnnotations{Title: "Search NVIDIA jobs", ReadOnlyHint: true},
		InputSchema: nvidiaSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *nvidiaSearchInput) (*mcp.CallToolResult, *nvidiaSearchOutput, error) {
		req, err := nvidiaMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.SearchJobs(ctx, req)
		if err != nil {
			if ue, ok := errors.AsType[*nvidia.ErrorResponseStatusCode](err); ok {
				return errorResult(fmt.Errorf("upstream error: %d", ue.StatusCode)), nil, nil
			}
			return errorResult(err), nil, nil
		}
		return nil, nvidiaHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "nvidia_get_job_detail",
		Description: "Get the full job description and requirements for an NVIDIA job by external path.",
		Annotations: &mcp.ToolAnnotations{Title: "Get NVIDIA job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *nvidiaDetailInput) (*mcp.CallToolResult, *nvidiaDetailOutput, error) {
		location, titleSlug, ok := nvidia.SplitExternalPath(in.ExternalPath)
		if !ok {
			return errorResult(fmt.Errorf("invalid external_path %q; must be in format '/job/{location}/{titleSlug}'", in.ExternalPath)), nil, nil
		}
		res, err := c.GetJobDetail(ctx, nvidia.GetJobDetailParams{
			Location:  location,
			TitleSlug: titleSlug,
		})
		if err != nil {
			if ue, ok := errors.AsType[*nvidia.ErrorResponseStatusCode](err); ok {
				return errorResult(fmt.Errorf("upstream error: %d", ue.StatusCode)), nil, nil
			}
			return errorResult(err), nil, nil
		}
		return nil, nvidiaHTTPToMCPDetail(res), nil
	})
}
