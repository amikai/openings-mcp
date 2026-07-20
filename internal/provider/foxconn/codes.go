package foxconn

// Code is one entry from a Foxconn filter enum: a query-parameter value and
// its zh-TW display name.
type Code struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// WorkplaceCodes are the valid `workplaceCode` list-filter values, hand-
// transcribed from GET /hh_recruit_tw_api/portal_api/Labels/Workplace/Codes
// (captured 2026-07-21), in the API's own display order. This enum changes
// rarely, so it is embedded as static reference data rather than fetched at
// runtime.
var WorkplaceCodes = []Code{
	{Code: "TA", Name: "台灣"},
	{Code: "CH", Name: "大陸"},
	{Code: "PH", Name: "菲律賓"},
	{Code: "HK", Name: "香港"},
	{Code: "IND", Name: "印尼"},
	{Code: "BZ", Name: "巴西"},
	{Code: "MC", Name: "墨西哥"},
	{Code: "ZK", Name: "捷克"},
	{Code: "VM", Name: "越南"},
	{Code: "JP", Name: "日本"},
	{Code: "AM", Name: "美國"},
	{Code: "ID", Name: "印度"},
	{Code: "OT", Name: "其他"},
}

// TalentZoneCodes are the valid `talentZoneCode` list-filter values, hand-
// transcribed from GET /hh_recruit_tw_api/portal_api/Labels/TalentZone/Codes
// (captured 2026-07-21), in the API's own display order. Some entries are
// dated recruitment-campaign zones (e.g. Y221209-01). Like WorkplaceCodes,
// this is embedded static reference data.
var TalentZoneCodes = []Code{
	{Code: "IAI_KS", Name: "高雄軟體研發中心專區"},
	{Code: "Y221209-01", Name: "電動車聯合招募專區"},
	{Code: "Y220606-01", Name: "半導體類專區"},
	{Code: "Y211129-01", Name: "軟體研發中心專區"},
	{Code: "Y211129-02", Name: "系統晶片設計專區"},
	{Code: "DR", Name: "身障友善職缺專區"},
	{Code: "TALENTS", Name: "一般招募(社招/顧問)"},
	{Code: "MA", Name: "新幹班"},
	{Code: "GMA", Name: "國際經營儲備幹部"},
	{Code: "INTERN", Name: "實習"},
	{Code: "PT", Name: "工讀生"},
	{Code: "IAI", Name: "工業互聯網/AI人才專區"},
	{Code: "Y230505-02", Name: "越南人才專區"},
	{Code: "Y230707-01", Name: "印度人才專區"},
}
