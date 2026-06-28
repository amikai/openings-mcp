package jobmcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/amikai/job-mcp/internal/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type tsmcSearchInput struct {
	Keyword         string   `json:"keyword" jsonschema:"search keyword, required"`
	Locations       []string `json:"locations,omitempty" jsonschema:"sites; any of: taiwan, canada, china, germany_dresden, germany_munich, japan_yokohama, japan_osaka, japan_tsukuba, japan_kumamoto, korea, netherlands, usa_arizona, usa_california, usa_massachusetts, usa_texas, usa_washington, usa_washington_dc"`
	Categories      []string `json:"categories,omitempty" jsonschema:"job families; any of: rd, specialty_technology, ic_design_technology, manufacturing, facility_and_safety, product_development, ic_packaging_technology, testing_development, quality_and_reliability, it, internal_audit, business_development, customer_service, corporate_planning, finance, human_resources, legal, materials_management, corporate_sustainability, administration, accessibility_inclusion"`
	JobTypes        []string `json:"job_types,omitempty" jsonschema:"seniority; any of: technician, associate_engineer, engineer, manager, others"`
	EmploymentTypes []string `json:"employment_types,omitempty" jsonschema:"any of: regular, temporary, intern, apprenticeship"`
	Page            int      `json:"page,omitempty" jsonschema:"1-based page number"`
}

type tsmcDetailInput struct {
	JobID string `json:"job_id" jsonschema:"tsmc job id (from search results), required"`
}

var (
	tsmcLocations = map[string]string{
		"taiwan":            tsmc.LocTaiwan,
		"canada":            tsmc.LocCanada,
		"china":             tsmc.LocChina,
		"germany_dresden":   tsmc.LocGermanyDresden,
		"germany_munich":    tsmc.LocGermanyMunich,
		"japan_yokohama":    tsmc.LocJapanYokohama,
		"japan_osaka":       tsmc.LocJapanOsaka,
		"japan_tsukuba":     tsmc.LocJapanTsukuba,
		"japan_kumamoto":    tsmc.LocJapanKumamoto,
		"korea":             tsmc.LocKorea,
		"netherlands":       tsmc.LocNetherlands,
		"usa_arizona":       tsmc.LocUSAArizona,
		"usa_california":    tsmc.LocUSACalifornia,
		"usa_massachusetts": tsmc.LocUSAMassachusetts,
		"usa_texas":         tsmc.LocUSATexas,
		"usa_washington":    tsmc.LocUSAWashington,
		"usa_washington_dc": tsmc.LocUSAWashingtonDC,
	}
	tsmcCategories = map[string]string{
		"rd":                       tsmc.CatRD,
		"specialty_technology":     tsmc.CatSpecialtyTechnology,
		"ic_design_technology":     tsmc.CatICDesignTechnology,
		"manufacturing":            tsmc.CatManufacturing,
		"facility_and_safety":      tsmc.CatFacilityAndSafety,
		"product_development":      tsmc.CatProductDevelopment,
		"ic_packaging_technology":  tsmc.CatICPackagingTechnology,
		"testing_development":      tsmc.CatTestingDevelopment,
		"quality_and_reliability":  tsmc.CatQualityAndReliability,
		"it":                       tsmc.CatIT,
		"internal_audit":           tsmc.CatInternalAudit,
		"business_development":     tsmc.CatBusinessDevelopment,
		"customer_service":         tsmc.CatCustomerService,
		"corporate_planning":       tsmc.CatCorporatePlanning,
		"finance":                  tsmc.CatFinance,
		"human_resources":          tsmc.CatHumanResources,
		"legal":                    tsmc.CatLegal,
		"materials_management":     tsmc.CatMaterialsManagement,
		"corporate_sustainability": tsmc.CatCorporateSustainability,
		"administration":           tsmc.CatAdministration,
		"accessibility_inclusion":  tsmc.CatAccessibilityInclusion,
	}
	tsmcJobTypes = map[string]string{
		"technician":         tsmc.JobTypeTechnician,
		"associate_engineer": tsmc.JobTypeAssociateEngineer,
		"engineer":           tsmc.JobTypeEngineer,
		"manager":            tsmc.JobTypeManager,
		"others":             tsmc.JobTypeOthers,
	}
	tsmcEmploymentTypes = map[string]string{
		"regular":        tsmc.EmployRegular,
		"temporary":      tsmc.EmployTemporary,
		"intern":         tsmc.EmployIntern,
		"apprenticeship": tsmc.EmployApprenticeship,
	}
)

// mapCodes translates human enum labels to tsmc facet codes, erroring on any unknown label.
func mapCodes(field string, labels []string, m map[string]string) ([]string, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		code, ok := m[l]
		if !ok {
			return nil, fmt.Errorf("invalid %s %q", field, l)
		}
		out = append(out, code)
	}
	return out, nil
}

func tsmcToRequest(in tsmcSearchInput) (*tsmc.JobRequest, error) {
	r := &tsmc.JobRequest{Keyword: in.Keyword, Page: in.Page}
	var err error
	if r.Locations, err = mapCodes("location", in.Locations, tsmcLocations); err != nil {
		return nil, err
	}
	if r.Categories, err = mapCodes("category", in.Categories, tsmcCategories); err != nil {
		return nil, err
	}
	if r.JobTypes, err = mapCodes("job_type", in.JobTypes, tsmcJobTypes); err != nil {
		return nil, err
	}
	if r.EmploymentTypes, err = mapCodes("employment_type", in.EmploymentTypes, tsmcEmploymentTypes); err != nil {
		return nil, err
	}
	return r, nil
}

func formatTSMCSearch(r *tsmc.SearchResponse) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d jobs (showing %d)\n\n", r.Total, len(r.Jobs))
	for i, j := range r.Jobs {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, j.ID, j.Title)
		fmt.Fprintf(&sb, "   Location: %s | Area: %s | %s | Posted: %s\n\n",
			j.Location, j.CareerArea, j.EmploymentType, j.Posted)
	}
	return sb.String()
}

func formatTSMCDetail(d *tsmc.JobDetail) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", d.Title)
	fmt.Fprintf(&sb, "Company: %s | Location: %s | Area: %s\n", d.Company, d.Location, d.CareerArea)
	fmt.Fprintf(&sb, "Type: %s | Employment: %s | Posted: %s\n\n", d.JobType, d.EmploymentType, d.Posted)
	fmt.Fprintf(&sb, "Responsibilities:\n%s\n\nQualifications:\n%s\n", d.Responsibilities, d.Qualifications)
	return sb.String()
}

// RegisterTSMC registers the tsmc search and job-detail tools.
func RegisterTSMC(s *mcp.Server, c *tsmc.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "tsmc_search_jobs",
		Description: "Search TSMC careers by keyword, with optional location/category/seniority/employment filters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in tsmcSearchInput) (*mcp.CallToolResult, any, error) {
		req, err := tsmcToRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(formatTSMCSearch(resp)), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tsmc_get_job_detail",
		Description: "Get the full TSMC job description for a job id (from search results).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in tsmcDetailInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.JobDetail(ctx, in.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(formatTSMCDetail(resp)), nil, nil
	})
}
