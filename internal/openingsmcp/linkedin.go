package openingsmcp

import (
	"fmt"
	"strings"

	"github.com/amikai/openings-mcp/internal/provider/linkedin"
)

var linkedinSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text search query matched against job title, company, and description."
		},
		"location": {
			"type": "string",
			"description": "Free-text location filter. LinkedIn searches globally; there is no separate country-code parameter."
		},
		"distance": {
			"type": "integer",
			"description": "Search radius in miles around location.",
			"minimum": 0
		},
		"workplace_type": {
			"type": "string",
			"description": "Workplace type filter.",
			"enum": ["On-site", "Remote", "Hybrid"]
		},
		"job_type": {
			"type": "string",
			"description": "Job type filter.",
			"enum": ["Full-time", "Part-time", "Contract", "Temporary", "Internship"]
		},
		"easy_apply": {
			"type": "boolean",
			"description": "Only jobs with LinkedIn Easy Apply."
		},
		"company_ids": {
			"type": "string",
			"description": "Comma-separated LinkedIn numeric company IDs. IDs are opaque and must be resolved from a company's public page or a prior search response, not guessed."
		},
		"posted_within": {
			"type": "string",
			"description": "Only jobs posted within this window.",
			"enum": ["Past day", "Past week", "Past month"]
		},
		"start": {
			"type": "integer",
			"description": "Zero-based result offset; default 0. The endpoint always returns exactly 10 cards per call regardless of this value, so paging through results must increment start by exactly 10 each call (0, 10, 20, ...) to avoid gaps. Do not mimic a real browser's 25-per-step scroll traffic, which skips 10 of every 25 positions this endpoint can return.",
			"minimum": 0
		}
	},
	"additionalProperties": false
}`)

// linkedinSearchInputSchema is hand-written JSON kept aligned with
// openapi.yaml's searchJobs parameters: human labels instead of the site's
// raw form-field codes (workplace_type/job_type map back via ids.go;
// posted_within maps back via linkedinPostedWithinSeconds below).
var linkedinSearchInputSchema = mustSchema(linkedinSearchInputRawSchema)

type linkedinSearchInput struct {
	Keyword       string `json:"keyword,omitempty"`
	Location      string `json:"location,omitempty"`
	Distance      int    `json:"distance,omitempty"`
	WorkplaceType string `json:"workplace_type,omitempty"`
	JobType       string `json:"job_type,omitempty"`
	EasyApply     bool   `json:"easy_apply,omitempty"`
	CompanyIDs    string `json:"company_ids,omitempty"`
	PostedWithin  string `json:"posted_within,omitempty"`
	Start         int    `json:"start,omitempty"`
}

// linkedinPostedWithinSeconds maps a human label to the seconds value
// linkedin.JobsRequest.PostedWithinSeconds expects (f_TPR=r{n} on the wire).
var linkedinPostedWithinSeconds = map[string]int{
	"Past day":   86400,
	"Past week":  604800,
	"Past month": 2592000,
}

func linkedinMCPToHTTPRequest(in *linkedinSearchInput) (*linkedin.JobsRequest, error) {
	req := &linkedin.JobsRequest{
		Keywords:  in.Keyword,
		Location:  in.Location,
		Distance:  in.Distance,
		EasyApply: in.EasyApply,
		Start:     in.Start,
	}

	if in.WorkplaceType != "" {
		id, ok := linkedin.WorkplaceTypeIDs[in.WorkplaceType]
		if !ok {
			return nil, fmt.Errorf("invalid workplace_type %q", in.WorkplaceType)
		}
		req.WorkplaceType = id
	}

	if in.JobType != "" {
		id, ok := linkedin.JobTypeIDs[in.JobType]
		if !ok {
			return nil, fmt.Errorf("invalid job_type %q", in.JobType)
		}
		req.JobType = id
	}

	if in.CompanyIDs != "" {
		for _, id := range strings.Split(in.CompanyIDs, ",") {
			if id = strings.TrimSpace(id); id != "" {
				req.CompanyIDs = append(req.CompanyIDs, id)
			}
		}
	}

	if in.PostedWithin != "" {
		seconds, ok := linkedinPostedWithinSeconds[in.PostedWithin]
		if !ok {
			return nil, fmt.Errorf("invalid posted_within %q", in.PostedWithin)
		}
		req.PostedWithinSeconds = seconds
	}

	return req, nil
}
