package job104

import (
	"fmt"
	"strings"
)

type Company struct {
	EncodedCustNo     string `json:"encodedCustNo"`
	Name              string `json:"name"`
	AreaDesc          string `json:"areaDesc"`
	IndustryDesc      string `json:"industryDesc"`
	CapitalDesc       string `json:"capitalDesc"`
	EmployeeCountDesc string `json:"employeeCountDesc"`
	Profile           string `json:"profile"`
	JobCount          int    `json:"jobCount"`
	IsSaved           bool   `json:"isSaved"`
}

type CompanyDetail struct {
	CustName      string `json:"custName"`
	CustNo        string `json:"custNo"`
	IndustryDesc  string `json:"industryDesc"`
	EmpNo         string `json:"empNo"`
	Capital       string `json:"capital"`
	Address       string `json:"address"`
	CustLink      string `json:"custLink"`
	Profile       string `json:"profile"`
	Product       string `json:"product"`
	Welfare       string `json:"welfare"`
	HRName        string `json:"hrName"`
	FollowerCount int    `json:"followerCount"`
	Follow        struct {
		IsSaved          bool `json:"isSaved"`
		IsTracked        bool `json:"isTracked"`
		FollowedJobCount int  `json:"followedJobCount"`
	} `json:"follow"`
}

type CompanyJob struct {
	JobName       string `json:"jobName"`
	EncodedJobNo  string `json:"encodedJobNo"`
	JobSalaryDesc string `json:"jobSalaryDesc"`
	JobAddrNoDesc string `json:"jobAddrNoDesc"`
}

type SearchCompaniesParams struct {
	Keyword  string
	Page     int // default 1
	PageSize int // default 10
}

type SearchCompanyResponse struct {
	Data     []Company `json:"data"`
	Metadata struct {
		Pagination struct {
			Total       int `json:"total"`
			CurrentPage int `json:"currentPage"`
			LastPage    int `json:"lastPage"`
		} `json:"pagination"`
	} `json:"metadata"`
}

type CompanyDetailResponse struct {
	Data CompanyDetail `json:"data"`
}

type CompanyJobsResponse struct {
	Data struct {
		List struct {
			TopJobs    []CompanyJob `json:"topJobs"`
			NormalJobs []CompanyJob `json:"normalJobs"`
		} `json:"list"`
	} `json:"data"`
}

func FormatSearchCompanyResponse(r *SearchCompanyResponse) string {
	p := r.Metadata.Pagination
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d companies (page %d/%d)\n\n", p.Total, p.CurrentPage, p.LastPage)
	for _, co := range r.Data {
		fmt.Fprintf(&sb, "[%s] %s\n", co.EncodedCustNo, co.Name)
		fmt.Fprintf(&sb, "  Industry: %s\n", co.IndustryDesc)
		fmt.Fprintf(&sb, "  Location: %s\n", co.AreaDesc)
		fmt.Fprintf(&sb, "  Employees: %s\n", co.EmployeeCountDesc)
		fmt.Fprintf(&sb, "  Capital: %s\n", co.CapitalDesc)
		fmt.Fprintf(&sb, "  Open jobs: %d\n", co.JobCount)
		if p := co.Profile; p != "" {
			if len(p) > 100 {
				p = p[:100] + "..."
			}
			fmt.Fprintf(&sb, "  About: %s\n", p)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func FormatCompanyDetail(r *CompanyDetailResponse, companyCode string, jobs *CompanyJobsResponse) string {
	d := r.Data
	var jobList []CompanyJob
	if jobs != nil {
		jobList = append(jobs.Data.List.TopJobs, jobs.Data.List.NormalJobs...)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "=== %s ===\n", d.CustName)
	fmt.Fprintf(&sb, "Industry: %s\n", d.IndustryDesc)
	fmt.Fprintf(&sb, "Employees: %s\n", d.EmpNo)
	fmt.Fprintf(&sb, "Capital: %s\n", d.Capital)
	fmt.Fprintf(&sb, "Address: %s\n", d.Address)
	if d.CustLink != "" {
		fmt.Fprintf(&sb, "Website: %s\n", d.CustLink)
	}
	if d.HRName != "" {
		fmt.Fprintf(&sb, "HR Contact: %s\n", d.HRName)
	}
	fmt.Fprintf(&sb, "Followers: %d\n", d.FollowerCount)
	sb.WriteByte('\n')

	if d.Profile != "" {
		fmt.Fprintf(&sb, "--- Company Profile ---\n%s\n\n", d.Profile)
	}
	if d.Product != "" {
		fmt.Fprintf(&sb, "--- Products/Services ---\n%s\n\n", d.Product)
	}
	if d.Welfare != "" {
		fmt.Fprintf(&sb, "--- Benefits ---\n%s\n\n", d.Welfare)
	}

	if len(jobList) > 0 {
		sb.WriteString("--- Current Openings ---\n")
		for _, job := range jobList {
			fmt.Fprintf(&sb, "[%s] %s\n", job.EncodedJobNo, job.JobName)
			fmt.Fprintf(&sb, "  Salary: %s\n", job.JobSalaryDesc)
			if job.JobAddrNoDesc != "" {
				fmt.Fprintf(&sb, "  Location: %s\n", job.JobAddrNoDesc)
			}
			sb.WriteByte('\n')
		}
	}

	fmt.Fprintf(&sb, "Profile URL: https://www.104.com.tw/company/%s\n", companyCode)
	return sb.String()
}
