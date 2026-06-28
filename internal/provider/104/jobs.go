package job104

import (
	"fmt"
	"strings"
)

type Job struct {
	JobNo    string `json:"jobNo"`
	JobName  string `json:"jobName"`
	CustName string `json:"custName"`
	CustNo   string `json:"custNo"`
	Link     struct {
		Job  string `json:"job"`
		Cust string `json:"cust"`
	} `json:"link"`
	SalaryHigh     int    `json:"salaryHigh"`
	SalaryLow      int    `json:"salaryLow"`
	JobAddrNoDesc  string `json:"jobAddrNoDesc"`
	AppearDate     string `json:"appearDate"`
	ApplyCnt       int    `json:"applyCnt"`
	RemoteWorkType int    `json:"remoteWorkType"`
}

type JobDetail struct {
	Header struct {
		JobName    string `json:"jobName"`
		CustName   string `json:"custName"`
		CustURL    string `json:"custUrl"`
		AppearDate string `json:"appearDate"`
		IsSaved    bool   `json:"isSaved"`
		IsApplied  bool   `json:"isApplied"`
	} `json:"header"`
	Contact struct {
		HRName string `json:"hrName"`
		Email  string `json:"email"`
		Reply  string `json:"reply"`
	} `json:"contact"`
	Condition struct {
		WorkExp   string   `json:"workExp"`
		Edu       string   `json:"edu"`
		Major     []string `json:"major"`
		Specialty []struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"specialty"`
	} `json:"condition"`
	Welfare struct {
		Welfare string `json:"welfare"`
	} `json:"welfare"`
	JobDetail struct {
		JobDescription string `json:"jobDescription"`
		JobCategory    []struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"jobCategory"`
		Salary        string `json:"salary"`
		SalaryMin     int    `json:"salaryMin"`
		SalaryMax     int    `json:"salaryMax"`
		JobType       int    `json:"jobType"`
		AddressRegion string `json:"addressRegion"`
		AddressDetail string `json:"addressDetail"`
		ManageResp    string `json:"manageResp"`
		NeedEmp       string `json:"needEmp"`
		RemoteWork    string `json:"remoteWork"`
	} `json:"jobDetail"`
	Industry  string `json:"industry"`
	Employees string `json:"employees"`
	CustNo    string `json:"custNo"`
}

const (
	AreaTaipei    = "6001001000"
	AreaNewTaipei = "6001002000"
	AreaTaoyuan   = "6001003000"
	AreaTaichung  = "6001004000"
	AreaTainan    = "6001005000"
	AreaKaohsiung = "6001006000"
)

type JobRequest struct {
	Keyword    string
	Area       string
	RO         *int // 0=full-time, 1=part-time
	Order      *int // 15=newest, 1=relevance
	Page       *int
	Edu        string
	RemoteWork *int   // 0=no remote, 1=partial, 2=full
	S9         string // experience codes, comma-separated
}

type SearchJobResponse struct {
	Data     []Job `json:"data"`
	Metadata struct {
		Pagination struct {
			CurrentPage int `json:"currentPage"`
			LastPage    int `json:"lastPage"`
			Total       int `json:"total"`
		} `json:"pagination"`
	} `json:"metadata"`
}

func FormatSearchJobResponse(r *SearchJobResponse) string {
	p := r.Metadata.Pagination
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d jobs (page %d/%d)\n\n", p.Total, p.CurrentPage, p.LastPage)
	for _, job := range r.Data {
		parts := strings.Split(job.Link.Job, "/")
		code := parts[len(parts)-1]
		salary := "面議"
		if job.SalaryLow > 0 && job.SalaryHigh > 0 {
			salary = fmt.Sprintf("%d~%d萬", job.SalaryLow/10000, job.SalaryHigh/10000)
		}
		fmt.Fprintf(&sb, "[%s] %s\n", code, job.JobName)
		fmt.Fprintf(&sb, "  Company: %s\n", job.CustName)
		fmt.Fprintf(&sb, "  Location: %s\n", job.JobAddrNoDesc)
		fmt.Fprintf(&sb, "  Salary: %s\n", salary)
		fmt.Fprintf(&sb, "  Posted: %s\n", job.AppearDate)
		fmt.Fprintf(&sb, "  Applied: %d people\n", job.ApplyCnt)
		if job.RemoteWorkType > 0 {
			remote := "Partial"
			if job.RemoteWorkType == 2 {
				remote = "Full"
			}
			fmt.Fprintf(&sb, "  Remote: %s\n", remote)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

type JobDetailResponse struct {
	Data JobDetail `json:"data"`
}

func FormatJobDetail(r *JobDetailResponse, jobCode string) string {
	d := r.Data
	var sb strings.Builder
	jd := d.JobDetail

	fmt.Fprintf(&sb, "=== %s ===\n", d.Header.JobName)
	fmt.Fprintf(&sb, "Company: %s (%s, %s)\n", d.Header.CustName, d.Industry, d.Employees)
	fmt.Fprintf(&sb, "Posted: %s\n\n", d.Header.AppearDate)

	sb.WriteString("--- Salary & Location ---\n")
	fmt.Fprintf(&sb, "Salary: %s\n", jd.Salary)
	fmt.Fprintf(&sb, "Location: %s %s\n", jd.AddressRegion, jd.AddressDetail)
	if jd.RemoteWork != "" {
		fmt.Fprintf(&sb, "Remote: %s\n", jd.RemoteWork)
	}
	jobType := "Full-time"
	if jd.JobType != 1 {
		jobType = "Part-time"
	}
	fmt.Fprintf(&sb, "Job type: %s\n", jobType)
	fmt.Fprintf(&sb, "Positions: %s\n", jd.NeedEmp)
	fmt.Fprintf(&sb, "Management: %s\n\n", jd.ManageResp)

	sb.WriteString("--- Requirements ---\n")
	fmt.Fprintf(&sb, "Experience: %s\n", d.Condition.WorkExp)
	fmt.Fprintf(&sb, "Education: %s\n", d.Condition.Edu)
	if len(d.Condition.Major) > 0 {
		fmt.Fprintf(&sb, "Major: %s\n", strings.Join(d.Condition.Major, ", "))
	}
	if len(d.Condition.Specialty) > 0 {
		descs := make([]string, len(d.Condition.Specialty))
		for i, s := range d.Condition.Specialty {
			descs[i] = s.Description
		}
		fmt.Fprintf(&sb, "Skills: %s\n", strings.Join(descs, ", "))
	}

	sb.WriteString("\n--- Job Categories ---\n")
	cats := make([]string, len(jd.JobCategory))
	for i, cat := range jd.JobCategory {
		cats[i] = cat.Description
	}
	fmt.Fprintf(&sb, "%s\n\n", strings.Join(cats, ", "))

	fmt.Fprintf(&sb, "--- Description ---\n%s\n\n", jd.JobDescription)

	welfare := d.Welfare.Welfare
	if welfare == "" {
		welfare = "(not provided)"
	}
	fmt.Fprintf(&sb, "--- Welfare ---\n%s\n\n", welfare)

	sb.WriteString("--- Contact ---\n")
	fmt.Fprintf(&sb, "HR: %s\n", d.Contact.HRName)
	if d.Contact.Email != "" {
		fmt.Fprintf(&sb, "Email: %s\n", d.Contact.Email)
	}
	fmt.Fprintf(&sb, "Response policy: %s\n\n", d.Contact.Reply)

	applied, saved := "No", "No"
	if d.Header.IsApplied {
		applied = "Yes"
	}
	if d.Header.IsSaved {
		saved = "Yes"
	}
	fmt.Fprintf(&sb, "Applied: %s | Saved: %s\n", applied, saved)
	fmt.Fprintf(&sb, "Company profile: %s\n", d.Header.CustURL)
	fmt.Fprintf(&sb, "Apply URL: https://www.104.com.tw/job/%s\n", jobCode)

	return sb.String()
}
