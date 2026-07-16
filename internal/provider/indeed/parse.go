package indeed

import (
	"strings"
	"time"
)

// salaryRange is the min/max pair shared by search and detail compensation
// shapes (genqlient generates distinct types per selection set).
type salaryRange struct {
	Min float64
	Max float64
}

type baseSalary struct {
	UnitOfWork string
	Range      salaryRange
}

func compensationFromParts(base *baseSalary, estimated *baseSalary, currency, estimatedCurrency string) *Compensation {
	bs := base
	cur := currency
	if bs == nil && estimated != nil {
		bs = estimated
		cur = estimatedCurrency
	}
	if bs == nil {
		return nil
	}
	comp := &Compensation{Currency: cur, Interval: strings.ToUpper(bs.UnitOfWork)}
	if bs.Range.Min != 0 {
		comp.MinAmount = bs.Range.Min
	}
	if bs.Range.Max != 0 {
		comp.MaxAmount = bs.Range.Max
	}
	// Still expose a Compensation when only unit/currency is set and min/max
	// are zero — callers treat zero amounts as "not disclosed" the same way
	// the prior *float64 path did when min/max were null.
	if comp.MinAmount == 0 && comp.MaxAmount == 0 && bs.UnitOfWork == "" && cur == "" {
		return nil
	}
	return comp
}

func dateFromEpochMillis(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}

func companyURLFromRelative(rel, siteBase string) string {
	if rel == "" {
		return ""
	}
	return siteBase + rel
}

// jobURL builds the Indeed-hosted posting page URL for a job key.
func jobURL(siteBase, key string) string {
	if key == "" {
		return ""
	}
	return siteBase + "/viewjob?jk=" + key
}

// rangeMinMax reads min/max from a genqlient RangeType interface value.
// Concrete types: Range (min+max), AtLeast (min), AtMost (max), Exactly (value→both).
func rangeMinMax(r any) salaryRange {
	if r == nil {
		return salaryRange{}
	}
	// Order matters: Range implements both GetMin and GetMax; check that
	// before the one-sided interfaces so Range is not misread as AtLeast.
	type minMax interface {
		GetMin() float64
		GetMax() float64
	}
	type exact interface {
		GetValue() float64
	}
	type minOnly interface {
		GetMin() float64
	}
	type maxOnly interface {
		GetMax() float64
	}
	switch v := r.(type) {
	case minMax:
		return salaryRange{Min: v.GetMin(), Max: v.GetMax()}
	case exact:
		val := v.GetValue()
		return salaryRange{Min: val, Max: val}
	case minOnly:
		return salaryRange{Min: v.GetMin()}
	case maxOnly:
		return salaryRange{Max: v.GetMax()}
	default:
		return salaryRange{}
	}
}

func jobFromSearch(j GetJobSearchJobSearchJobSearchConnectionResultsJobSearchResultJob, siteBase, country string) Job {
	company, companyURL := "", ""
	if j.Employer != nil {
		company = j.Employer.Name
		companyURL = companyURLFromRelative(j.Employer.RelativeCompanyPageUrl, siteBase)
	}
	var types []string
	if len(j.Attributes) > 0 {
		types = make([]string, 0, len(j.Attributes))
		for _, a := range j.Attributes {
			types = append(types, a.Label)
		}
	}
	var base, estimated *baseSalary
	if j.Compensation.BaseSalary != nil {
		base = &baseSalary{
			UnitOfWork: j.Compensation.BaseSalary.UnitOfWork,
			Range:      rangeMinMax(j.Compensation.BaseSalary.Range),
		}
	}
	estCurrency := ""
	if j.Compensation.Estimated != nil {
		estCurrency = j.Compensation.Estimated.CurrencyCode
		if j.Compensation.Estimated.BaseSalary != nil {
			estimated = &baseSalary{
				UnitOfWork: j.Compensation.Estimated.BaseSalary.UnitOfWork,
				Range:      rangeMinMax(j.Compensation.Estimated.BaseSalary.Range),
			}
		}
	}
	return Job{
		Key:          j.Key,
		Title:        j.Title,
		Company:      company,
		CompanyURL:   companyURL,
		Location:     j.Location.Formatted.Long,
		Country:      country,
		JobURL:       jobURL(siteBase, j.Key),
		PostedDate:   dateFromEpochMillis(j.DatePublished),
		JobTypes:     types,
		Compensation: compensationFromParts(base, estimated, j.Compensation.CurrencyCode, estCurrency),
	}
}

func jobDetailFromDetail(j GetJobDetailJobDataJobDataConnectionResultsJobDataResultJob, siteBase string) JobDetail {
	detail := JobDetail{
		Key:         j.Key,
		Title:       j.Title,
		JobURL:      jobURL(siteBase, j.Key),
		PostedDate:  dateFromEpochMillis(j.DatePublished),
		DateIndexed: dateFromEpochMillis(j.DateOnIndeed),
		Location: Location{
			Country:       j.Location.CountryName,
			CountryCode:   j.Location.CountryCode,
			State:         j.Location.Admin1Code,
			City:          j.Location.City,
			PostalCode:    j.Location.PostalCode,
			StreetAddress: j.Location.StreetAddress,
			Formatted:     j.Location.Formatted.Long,
		},
	}
	if j.Description != nil {
		detail.Description = j.Description.Html
	}
	if j.Source != nil {
		detail.Source = j.Source.Name
	}
	if len(j.Attributes) > 0 {
		detail.JobTypes = make([]string, 0, len(j.Attributes))
		for _, a := range j.Attributes {
			detail.JobTypes = append(detail.JobTypes, a.Label)
		}
	}
	var base, estimated *baseSalary
	if j.Compensation.BaseSalary != nil {
		base = &baseSalary{
			UnitOfWork: j.Compensation.BaseSalary.UnitOfWork,
			Range:      rangeMinMax(j.Compensation.BaseSalary.Range),
		}
	}
	estCurrency := ""
	if j.Compensation.Estimated != nil {
		estCurrency = j.Compensation.Estimated.CurrencyCode
		if j.Compensation.Estimated.BaseSalary != nil {
			estimated = &baseSalary{
				UnitOfWork: j.Compensation.Estimated.BaseSalary.UnitOfWork,
				Range:      rangeMinMax(j.Compensation.Estimated.BaseSalary.Range),
			}
		}
	}
	detail.Compensation = compensationFromParts(base, estimated, j.Compensation.CurrencyCode, estCurrency)

	if j.Employer != nil {
		detail.Company = j.Employer.Name
		detail.CompanyURL = companyURLFromRelative(j.Employer.RelativeCompanyPageUrl, siteBase)
		if j.Employer.Dossier != nil {
			d := j.Employer.Dossier
			if d.EmployerDetails != nil {
				detail.CompanyIndustry = d.EmployerDetails.Industry
				detail.CompanyEmployees = d.EmployerDetails.EmployeesLocalizedLabel
				detail.CompanyRevenue = d.EmployerDetails.RevenueLocalizedLabel
				detail.CompanyDescription = d.EmployerDetails.BriefDescription
				detail.CompanyAddresses = d.EmployerDetails.Addresses
				detail.CompanyCEO = d.EmployerDetails.CeoName
				detail.CompanyCEOPhoto = d.EmployerDetails.CeoPhotoUrl
			}
			if d.Images != nil {
				detail.CompanyLogo = d.Images.SquareLogoUrl
				detail.CompanyBannerImage = d.Images.HeaderImageUrl
			}
			if d.Links != nil {
				detail.CompanyWebsite = d.Links.CorporateWebsite
			}
		}
	}
	if j.Recruit != nil {
		detail.ApplyURL = j.Recruit.ViewJobUrl
		detail.DetailedSalary = j.Recruit.DetailedSalary
		detail.WorkSchedule = j.Recruit.WorkSchedule
	}
	return detail
}
