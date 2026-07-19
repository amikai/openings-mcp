package meta

import "encoding/json"

type gqlEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

type gqlError struct {
	Message string `json:"message"`
}

type searchData struct {
	JobSearch *searchResult `json:"job_search_with_featured_jobs"`
}

type searchResult struct {
	AllJobs      []Job `json:"all_jobs"`
	FeaturedJobs []Job `json:"featured_jobs"`
}

type filtersData struct {
	Filters *wireFilters `json:"job_search_filters"`
}

type wireFilters struct {
	Teams        []wireTeam `json:"teams"`
	Technologies []string   `json:"technologies"`
	Roles        []string   `json:"roles"`
}

type wireTeam struct {
	DisplayName string `json:"team_display_name"`
}

type locationsData struct {
	Filters *wireLocations `json:"job_search_filters"`
}

type wireLocations struct {
	Locations []Location `json:"locations"`
}

type detailData struct {
	Description *wireJobDetail `json:"xcp_requisition_job_description"`
}

// htmlBlock unwraps the double-encoded rich-text wrapper: the field value is
// a JSON string whose content is itself a JSON object {"__html": "..."}.
type htmlBlock struct {
	HTML string
}

func (b *htmlBlock) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	var inner struct {
		HTML string `json:"__html"`
	}
	if err := json.Unmarshal([]byte(s), &inner); err != nil {
		return err
	}
	b.HTML = inner.HTML
	return nil
}

func (b *htmlBlock) or(fallback string) string {
	if b == nil {
		return fallback
	}
	return b.HTML
}

// listItem unwraps the {"item": "..."} wrapper used by qualification and
// responsibility lists.
type listItem struct {
	Item string `json:"item"`
}

func itemStrings(items []listItem) []string {
	if items == nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Item)
	}
	return out
}

type wireJobDetail struct {
	ID                              string          `json:"id"`
	Title                           string          `json:"title"`
	Locations                       []string        `json:"locations"`
	Departments                     []string        `json:"departments"`
	InternalDepartments             []string        `json:"internal_departments"`
	Description                     *htmlBlock      `json:"description"`
	MinimumQualifications           []listItem      `json:"minimum_qualifications"`
	PreferredQualifications         []listItem      `json:"preferred_qualifications"`
	Responsibilities                []listItem      `json:"responsibilities"`
	PublicCompensation              []Compensation  `json:"public_compensation"`
	ShowPartialPublicCompDisclaimer bool            `json:"show_partial_public_comp_disclaimer"`
	BoilerplateIntro                *htmlBlock      `json:"boiler_plate_intro"`
	CaliforniaDisclaimer            *htmlBlock      `json:"california_disclaimer"`
	IntlDisclaimer                  *htmlBlock      `json:"intl_disclaimer"`
	EqualOpportunityMessage         *htmlBlock      `json:"equal_opportunity_message"`
	AccommodationsMessage           *htmlBlock      `json:"accommodations_message"`
	OwnershipInformation            json.RawMessage `json:"ownership_information"`
}

func (w *wireJobDetail) toJobDetail() *JobDetail {
	return &JobDetail{
		ID:                              w.ID,
		Title:                           w.Title,
		Locations:                       w.Locations,
		Departments:                     w.Departments,
		InternalDepartments:             w.InternalDepartments,
		DescriptionHTML:                 w.Description.or(""),
		MinimumQualifications:           itemStrings(w.MinimumQualifications),
		PreferredQualifications:         itemStrings(w.PreferredQualifications),
		Responsibilities:                itemStrings(w.Responsibilities),
		PublicCompensation:              w.PublicCompensation,
		ShowPartialPublicCompDisclaimer: w.ShowPartialPublicCompDisclaimer,
		BoilerplateIntroHTML:            w.BoilerplateIntro.or(""),
		CaliforniaDisclaimerHTML:        w.CaliforniaDisclaimer.or(""),
		IntlDisclaimerHTML:              w.IntlDisclaimer.or(""),
		EqualOpportunityMessageHTML:     w.EqualOpportunityMessage.or(""),
		AccommodationsMessageHTML:       w.AccommodationsMessage.or(""),
		OwnershipInformation:            w.OwnershipInformation,
	}
}
