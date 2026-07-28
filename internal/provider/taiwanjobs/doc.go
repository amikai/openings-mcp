// Package taiwanjobs fetches public job listings from TaiwanJobs (台灣就業通),
// the Taiwan Ministry of Labor's official job-matching service, via its open
// XML feed published on the government open-data platform
// (https://data.gov.tw/dataset/44062).
//
// The feed is a single keyless GET endpoint (Webservice.ashx) with three
// documented query parameters: count (row cap, upstream max 1000), zipno
// (Taiwan postal code prefix for the work location), and jobno (official job
// category code, 2-digit major or 6-digit minor). There is no keyword
// parameter, no pagination, and no separate detail endpoint — each row
// already carries the full posting body (工作內容), salary range, education
// requirement, application deadline, and a link to the public listing page.
// Keyword narrowing is therefore done client-side over the fetched rows (see
// JobsRequest.Keyword).
//
// Element names in the upstream XML embed Chinese annotations, e.g.
// <OCCU_DESC（職務名稱）>; those exact names (fullwidth parentheses included)
// are mirrored in the struct tags rather than re-normalized upstream.
package taiwanjobs
