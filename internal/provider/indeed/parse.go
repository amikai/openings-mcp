package indeed

import (
	"strings"
	"time"
)

type wireGraphQLError struct {
	Message string `json:"message"`
}

type wireRange struct {
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
}

type wireBaseSalary struct {
	UnitOfWork string    `json:"unitOfWork"`
	Range      wireRange `json:"range"`
}

type wireCompensation struct {
	Estimated *struct {
		CurrencyCode string          `json:"currencyCode"`
		BaseSalary   *wireBaseSalary `json:"baseSalary"`
	} `json:"estimated"`
	BaseSalary   *wireBaseSalary `json:"baseSalary"`
	CurrencyCode string          `json:"currencyCode"`
}

type wireAttribute struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type wireLocation struct {
	CountryName   string `json:"countryName"`
	CountryCode   string `json:"countryCode"`
	Admin1Code    string `json:"admin1Code"`
	City          string `json:"city"`
	PostalCode    string `json:"postalCode"`
	StreetAddress string `json:"streetAddress"`
	Formatted     struct {
		Short string `json:"short"`
		Long  string `json:"long"`
	} `json:"formatted"`
}

type wireEmployer struct {
	RelativeCompanyPageURL string `json:"relativeCompanyPageUrl"`
	Name                   string `json:"name"`
	Dossier                *struct {
		EmployerDetails *struct {
			Addresses               []string `json:"addresses"`
			Industry                string   `json:"industry"`
			EmployeesLocalizedLabel string   `json:"employeesLocalizedLabel"`
			RevenueLocalizedLabel   string   `json:"revenueLocalizedLabel"`
			BriefDescription        string   `json:"briefDescription"`
			CEOName                 string   `json:"ceoName"`
			CEOPhotoURL             string   `json:"ceoPhotoUrl"`
		} `json:"employerDetails"`
		Images *struct {
			HeaderImageURL string `json:"headerImageUrl"`
			SquareLogoURL  string `json:"squareLogoUrl"`
		} `json:"images"`
		Links *struct {
			CorporateWebsite string `json:"corporateWebsite"`
		} `json:"links"`
	} `json:"dossier"`
}

type wireRecruit struct {
	ViewJobURL     string `json:"viewJobUrl"`
	DetailedSalary string `json:"detailedSalary"`
	WorkSchedule   string `json:"workSchedule"`
}

type wireJob struct {
	Key           string `json:"key"`
	Title         string `json:"title"`
	DatePublished int64  `json:"datePublished"`
	DateOnIndeed  int64  `json:"dateOnIndeed"`
	Description   *struct {
		HTML string `json:"html"`
	} `json:"description"`
	Location     wireLocation     `json:"location"`
	Compensation wireCompensation `json:"compensation"`
	Attributes   []wireAttribute  `json:"attributes"`
	Employer     *wireEmployer    `json:"employer"`
	Recruit      *wireRecruit     `json:"recruit"`
	Source       *struct {
		Name string `json:"name"`
	} `json:"source"`
}

type wireSearchResult struct {
	TrackingKey string  `json:"trackingKey"`
	Job         wireJob `json:"job"`
}

type wireSearchResponse struct {
	Data struct {
		JobSearch *struct {
			PageInfo struct {
				NextCursor *string `json:"nextCursor"`
			} `json:"pageInfo"`
			Results []wireSearchResult `json:"results"`
		} `json:"jobSearch"`
	} `json:"data"`
	Errors []wireGraphQLError `json:"errors"`
}

type wireDetailResponse struct {
	Data struct {
		JobData *struct {
			Results []struct {
				Job wireJob `json:"job"`
			} `json:"results"`
		} `json:"jobData"`
	} `json:"data"`
	Errors []wireGraphQLError `json:"errors"`
}

func compensationFromWire(c wireCompensation) *Compensation {
	bs := c.BaseSalary
	currency := c.CurrencyCode
	if bs == nil && c.Estimated != nil {
		bs = c.Estimated.BaseSalary
		currency = c.Estimated.CurrencyCode
	}
	if bs == nil {
		return nil
	}
	comp := &Compensation{Currency: currency, Interval: strings.ToUpper(bs.UnitOfWork)}
	if bs.Range.Min != nil {
		comp.MinAmount = int(*bs.Range.Min)
	}
	if bs.Range.Max != nil {
		comp.MaxAmount = int(*bs.Range.Max)
	}
	return comp
}

func jobTypesFromAttributes(attrs []wireAttribute) []string {
	if len(attrs) == 0 {
		return nil
	}
	types := make([]string, 0, len(attrs))
	for _, a := range attrs {
		types = append(types, a.Label)
	}
	return types
}

func dateFromEpochMillis(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}

func companyURLFromRelative(rel, siteBaseURL string) string {
	if rel == "" {
		return ""
	}
	return siteBaseURL + rel
}

func employerName(e *wireEmployer) string {
	if e == nil {
		return ""
	}
	return e.Name
}

func employerCompanyURL(e *wireEmployer, siteBaseURL string) string {
	if e == nil {
		return ""
	}
	return companyURLFromRelative(e.RelativeCompanyPageURL, siteBaseURL)
}

func locationFromWire(l wireLocation) Location {
	return Location{
		Country:       l.CountryName,
		CountryCode:   l.CountryCode,
		State:         l.Admin1Code,
		City:          l.City,
		PostalCode:    l.PostalCode,
		StreetAddress: l.StreetAddress,
		Formatted:     l.Formatted.Long,
	}
}

func sourceName(s *struct {
	Name string `json:"name"`
}) string {
	if s == nil {
		return ""
	}
	return s.Name
}

func jobFromWire(w wireJob, siteBaseURL string) Job {
	return Job{
		Key:          w.Key,
		Title:        w.Title,
		Company:      employerName(w.Employer),
		CompanyURL:   employerCompanyURL(w.Employer, siteBaseURL),
		Location:     w.Location.Formatted.Long,
		JobURL:       jobURL(siteBaseURL, w.Key),
		PostedDate:   dateFromEpochMillis(w.DatePublished),
		JobTypes:     jobTypesFromAttributes(w.Attributes),
		Compensation: compensationFromWire(w.Compensation),
	}
}

func jobDetailFromWire(w wireJob, siteBaseURL string) JobDetail {
	detail := JobDetail{
		Key:          w.Key,
		Title:        w.Title,
		Company:      employerName(w.Employer),
		CompanyURL:   employerCompanyURL(w.Employer, siteBaseURL),
		Location:     locationFromWire(w.Location),
		JobURL:       jobURL(siteBaseURL, w.Key),
		PostedDate:   dateFromEpochMillis(w.DatePublished),
		DateIndexed:  dateFromEpochMillis(w.DateOnIndeed),
		Source:       sourceName(w.Source),
		JobTypes:     jobTypesFromAttributes(w.Attributes),
		Compensation: compensationFromWire(w.Compensation),
	}
	if w.Description != nil {
		detail.Description = w.Description.HTML
	}
	if w.Recruit != nil {
		detail.ApplyURL = w.Recruit.ViewJobURL
		detail.DetailedSalary = w.Recruit.DetailedSalary
		detail.WorkSchedule = w.Recruit.WorkSchedule
	}
	if w.Employer != nil && w.Employer.Dossier != nil {
		d := w.Employer.Dossier
		if d.EmployerDetails != nil {
			detail.CompanyIndustry = d.EmployerDetails.Industry
			detail.CompanyEmployees = d.EmployerDetails.EmployeesLocalizedLabel
			detail.CompanyRevenue = d.EmployerDetails.RevenueLocalizedLabel
			detail.CompanyDescription = d.EmployerDetails.BriefDescription
			detail.CompanyAddresses = d.EmployerDetails.Addresses
			detail.CompanyCEO = d.EmployerDetails.CEOName
			detail.CompanyCEOPhoto = d.EmployerDetails.CEOPhotoURL
		}
		if d.Images != nil {
			detail.CompanyLogo = d.Images.SquareLogoURL
			detail.CompanyBannerImage = d.Images.HeaderImageURL
		}
		if d.Links != nil {
			detail.CompanyWebsite = d.Links.CorporateWebsite
		}
	}
	return detail
}

// jobURL builds the Indeed-hosted posting page URL for a job key.
func jobURL(siteBaseURL, key string) string {
	if key == "" {
		return ""
	}
	return siteBaseURL + "/viewjob?jk=" + key
}
