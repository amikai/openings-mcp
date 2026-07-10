package job104

// AreaIDs maps an area label to its 104 area code. Codes are opaque and not
// sequential/guessable from the value (confirmed live — one wrong guess
// pointed at Yilan County instead of Taoyuan). Sourced from 104's own
// region-picker data at https://static.104.com.tw/category-tool/json/Area.json,
// which covers Taiwan's 20 counties/cities plus every overseas region (mainland
// China, other Asia, Oceania, US/Canada, Central/South America, Europe,
// Africa) the site's job search lets you filter by. Only the top two levels
// (region group + county/country) are included; finer district-level entries
// (e.g. Taipei's individual districts) are not.
var AreaIDs = map[string]SearchJobsArea{
	// Taiwan
	"Taipei":     SearchJobsArea("6001001000"),
	"NewTaipei":  SearchJobsArea("6001002000"),
	"Yilan":      SearchJobsArea("6001003000"),
	"Keelung":    SearchJobsArea("6001004000"),
	"Taoyuan":    SearchJobsArea("6001005000"),
	"Hsinchu":    SearchJobsArea("6001006000"),
	"Miaoli":     SearchJobsArea("6001007000"),
	"Taichung":   SearchJobsArea("6001008000"),
	"Changhua":   SearchJobsArea("6001010000"),
	"Nantou":     SearchJobsArea("6001011000"),
	"Yunlin":     SearchJobsArea("6001012000"),
	"Chiayi":     SearchJobsArea("6001013000"),
	"Tainan":     SearchJobsArea("6001014000"),
	"Kaohsiung":  SearchJobsArea("6001016000"),
	"Pingtung":   SearchJobsArea("6001018000"),
	"Taitung":    SearchJobsArea("6001019000"),
	"Hualien":    SearchJobsArea("6001020000"),
	"Penghu":     SearchJobsArea("6001021000"),
	"Kinmen":     SearchJobsArea("6001022000"),
	"Lienchiang": SearchJobsArea("6001023000"),

	// Mainland China
	"Beijing":       SearchJobsArea("6002001000"),
	"Tianjin":       SearchJobsArea("6002002000"),
	"Shanghai":      SearchJobsArea("6002003000"),
	"Chongqing":     SearchJobsArea("6002004000"),
	"Guangdong":     SearchJobsArea("6002005000"),
	"Fujian":        SearchJobsArea("6002006000"),
	"Hainan":        SearchJobsArea("6002007000"),
	"Zhejiang":      SearchJobsArea("6002008000"),
	"Jiangsu":       SearchJobsArea("6002009000"),
	"Shandong":      SearchJobsArea("6002010000"),
	"Hebei":         SearchJobsArea("6002011000"),
	"Liaoning":      SearchJobsArea("6002012000"),
	"Jilin":         SearchJobsArea("6002013000"),
	"Heilongjiang":  SearchJobsArea("6002014000"),
	"Hunan":         SearchJobsArea("6002015000"),
	"Hubei":         SearchJobsArea("6002016000"),
	"Jiangxi":       SearchJobsArea("6002017000"),
	"Anhui":         SearchJobsArea("6002018000"),
	"Henan":         SearchJobsArea("6002019000"),
	"Shanxi":        SearchJobsArea("6002020000"),
	"Shaanxi":       SearchJobsArea("6002021000"),
	"Gansu":         SearchJobsArea("6002022000"),
	"Qinghai":       SearchJobsArea("6002023000"),
	"Sichuan":       SearchJobsArea("6002024000"),
	"Guizhou":       SearchJobsArea("6002025000"),
	"Yunnan":        SearchJobsArea("6002026000"),
	"InnerMongolia": SearchJobsArea("6002027000"),
	"Tibet":         SearchJobsArea("6002028000"),
	"Ningxia":       SearchJobsArea("6002029000"),
	"Xinjiang":      SearchJobsArea("6002030000"),
	"Guangxi":       SearchJobsArea("6002031000"),
	"HongKong":      SearchJobsArea("6002032000"),
	"Macao":         SearchJobsArea("6002033000"),

	// Other Asia
	"NortheastAsia": SearchJobsArea("6003001000"),
	"SoutheastAsia": SearchJobsArea("6003002000"),
	"OtherAsia":     SearchJobsArea("6003003000"),

	// Oceania
	"AustraliaNZ":  SearchJobsArea("6004001000"),
	"OtherOceania": SearchJobsArea("6004002000"),

	// US / Canada
	"Canada":       SearchJobsArea("6005001000"),
	"EasternUS":    SearchJobsArea("6005002000"),
	"WesternUS":    SearchJobsArea("6005003000"),
	"MidwesternUS": SearchJobsArea("6005004000"),

	// Central / South America
	"CentralAmerica": SearchJobsArea("6006001000"),
	"SouthAmerica":   SearchJobsArea("6006002000"),

	// Europe
	"NorthernEurope": SearchJobsArea("6007001000"),
	"SouthernEurope": SearchJobsArea("6007002000"),
	"EasternEurope":  SearchJobsArea("6007003000"),
	"WesternEurope":  SearchJobsArea("6007004000"),
	"CentralEurope":  SearchJobsArea("6007005000"),

	// Africa
	"NorthAfrica":   SearchJobsArea("6008001000"),
	"CentralAfrica": SearchJobsArea("6008002000"),
	"SouthAfrica":   SearchJobsArea("6008003000"),
	"EastAfrica":    SearchJobsArea("6008004000"),
	"WestAfrica":    SearchJobsArea("6008005000"),
}

// RoIDs maps a "Job type" label to its ro request value. See
// SearchJobs's `ro` parameter description in openapi.yaml for how these
// were confirmed live against the site's own filter UI.
var RoIDs = map[string]SearchJobsRo{
	"Full-time": SearchJobsRo1,
	"Part-time": SearchJobsRo2,
	"Senior":    SearchJobsRo3,
	"Dispatch":  SearchJobsRo4,
}

// OrderIDs maps a sort-order label to its order request value. See
// SearchJobs's `order` parameter description in openapi.yaml — `15` was a
// wrong prior guess for Newest; `2` is the confirmed value. SalaryHigh is
// not a pure sort: it also drops undisclosed-salary (待遇面議) postings
// from the results and total.
var OrderIDs = map[string]SearchJobsOrder{
	"Relevance":  SearchJobsOrder1,
	"Newest":     SearchJobsOrder2,
	"SalaryHigh": SearchJobsOrder13,
}

// JobExpIDs maps a minimum-experience-bracket label to its jobexp request
// value. The server rejects any other value with a 422, and the five
// brackets are disjoint — comma-joined multi-selects union like `edu`. See
// SearchJobs's `jobexp` parameter description in openapi.yaml.
var JobExpIDs = map[string]SearchJobsJobexpItem{
	"Under1Year":  SearchJobsJobexpItem1,
	"1To3Years":   SearchJobsJobexpItem3,
	"3To5Years":   SearchJobsJobexpItem5,
	"5To10Years":  SearchJobsJobexpItem10,
	"Over10Years": SearchJobsJobexpItem99,
}

// RemoteWorkIDs maps a remote-work label to its remoteWork request value.
var RemoteWorkIDs = map[string]SearchJobsRemoteWork{
	"Full":    SearchJobsRemoteWork1,
	"Partial": SearchJobsRemoteWork2,
}

// EduIDs maps an education-level label to its edu request value.
var EduIDs = map[string]SearchJobsEduItem{
	"HighSchoolBelow": SearchJobsEduItem1,
	"HighSchool":      SearchJobsEduItem2,
	"College":         SearchJobsEduItem3,
	"University":      SearchJobsEduItem4,
	"Master":          SearchJobsEduItem5,
	"Doctorate":       SearchJobsEduItem6,
}

// S9IDs maps a shift-type label to its s9 request value. Despite the
// underlying codes being powers of two (1/2/4/8), the server rejects
// OR'd values (e.g. 3, 6) — only these four exact values round-trip, joined
// with commas for a multi-select.
var S9IDs = map[string]SearchJobsS9Item{
	"Day":       SearchJobsS9Item1,
	"Night":     SearchJobsS9Item2,
	"Graveyard": SearchJobsS9Item4,
	"Holiday":   SearchJobsS9Item8,
}
