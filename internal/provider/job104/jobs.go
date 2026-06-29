package job104

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

type JobsRequest struct {
	Keyword    string
	Area       string
	RO         *int // 0=full-time, 1=part-time
	Order      *int // 15=newest, 1=relevance
	Page       *int
	Edu        string
	RemoteWork *int   // 0=no remote, 1=partial, 2=full
	S9         string // experience codes, comma-separated
}

type JobsResponse struct {
	Data     []Job `json:"data"`
	Metadata struct {
		Pagination struct {
			CurrentPage int `json:"currentPage"`
			LastPage    int `json:"lastPage"`
			Total       int `json:"total"`
		} `json:"pagination"`
	} `json:"metadata"`
}

type JobDetailResponse struct {
	Data JobDetail `json:"data"`
}
