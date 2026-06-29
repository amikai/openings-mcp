package job104

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

type CompaniesRequest struct {
	Keyword  string
	Page     int // default 1
	PageSize int // default 10
}

type CompaniesResponse struct {
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

