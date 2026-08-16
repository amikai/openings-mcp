package mediatek

// FilterOption is a human-readable label and the opaque value accepted by
// MediaTek's public search API.
type FilterOption struct {
	Label string
	Code  string
}

var CategoryOptions = []FilterOption{
	{Label: "Adminstrator", Code: "9001"},
	{Label: "Algorithm & Architecture", Code: "9002"},
	{Label: "Analog_RF", Code: "9029"},
	{Label: "Application & Customer Support", Code: "9003"},
	{Label: "Audit", Code: "9007"},
	{Label: "Chip Design", Code: "9009"},
	{Label: "Finance", Code: "9010"},
	{Label: "General Management", Code: "9011"},
	{Label: "Human Resources", Code: "9012"},
	{Label: "Information Technology", Code: "9013"},
	{Label: "Legal & Patent", Code: "9014"},
	{Label: "Manufacturing & Quality", Code: "9015"},
	{Label: "Marcom", Code: "9016"},
	{Label: "Process Technology Development", Code: "9017"},
	{Label: "SQA & Testing", Code: "9021"},
	{Label: "Sales & Marketing", Code: "9019"},
	{Label: "Software", Code: "9020"},
	{Label: "System Application", Code: "9022"},
	{Label: "Technology/Project Management", Code: "9023"},
}

var WorkExperienceOptions = []FilterOption{
	{Label: "Less than 2 Years Work Expe.", Code: "0002"},
	{Label: "More than 2 Years Work Expe.", Code: "0003"},
	{Label: "More than 4 Years Work Expe.", Code: "0004"},
	{Label: "More than 6 Years Work Expe.", Code: "0005"},
	{Label: "More than 8 Years Work Expe.", Code: "0006"},
	{Label: "No Work Expe.", Code: "0011"},
}

var LocationOptions = []FilterOption{
	{Label: "Dubai", Code: "0000040950"},
	{Label: "Australia", Code: "9029"},
	{Label: "Beijing", Code: "0000009291"},
	{Label: "Hefei", Code: "0000009293"},
	{Label: "Shenzhen", Code: "0000009294"},
	{Label: "Chengdu", Code: "0000019031"},
	{Label: "Wuhan", Code: "0000040453"},
	{Label: "Shanghai", Code: "0000040455"},
	{Label: "Düsseldorf", Code: "9028"},
	{Label: "Aalborg", Code: "0000009301"},
	{Label: "Cairo", Code: "9041"},
	{Label: "Oulu", Code: "0000182103"},
	{Label: "Cambourne", Code: "0000009302"},
	{Label: "Kent", Code: "0000009303"},
	{Label: "West Malling", Code: "9026"},
	{Label: "London", Code: "9042"},
	{Label: "Noida", Code: "0000009297"},
	{Label: "Bangalore", Code: "0000168800"},
	{Label: "Mumbai", Code: "9021"},
	{Label: "Tokyo", Code: "0000188587"},
	{Label: "Seongnam", Code: "0000009298"},
	{Label: "Seoul", Code: "9040"},
	{Label: "Warsaw", Code: "9030"},
	{Label: "REMOTE-Poland", Code: "9032"},
	{Label: "Linköping", Code: "0000096722"},
	{Label: "Stockholm", Code: "0000155756"},
	{Label: "Singapore", Code: "0000009296"},
	{Label: "Taipei", Code: "0000009255"},
	{Label: "HsinChu", Code: "0000009256"},
	{Label: "ChuPei", Code: "0000073451"},
	{Label: "Taipei (XS)", Code: "9020"},
	{Label: "Tainan", Code: "9031"},
	{Label: "San Jose, CA", Code: "0000009305"},
	{Label: "Woburn, MA", Code: "0000120307"},
	{Label: "Austin, TX", Code: "0000120308"},
	{Label: "Irvine, CA", Code: "0000142403"},
	{Label: "San Diego, CA", Code: "0000142856"},
	{Label: "Warren, NJ", Code: "9034"},
	{Label: "Dallas, TX", Code: "9036"},
	{Label: "Bellevue, WA", Code: "9037"},
	{Label: "West Lafayette, IN", Code: "9038"},
	{Label: "Portland, OR", Code: "9043"},
}

var ProgramOptions = []FilterOption{
	{Label: "5G", Code: "20"},
	{Label: "RDSS", Code: "30"},
	{Label: "Intern", Code: "40"},
	{Label: "AI", Code: "50"},
	{Label: "Hardware Talents Referral Program", Code: "60"},
}

func optionCode(options []FilterOption, label string) (string, bool) {
	for _, option := range options {
		if option.Label == label {
			return option.Code, true
		}
	}
	return "", false
}
