# Category (fieldName 5) filter value is the department id from GetFilters'
# ViewMoreJobCategories view-more call, not a display label.
POST https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/JobBoardView/LoadSearchResults
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Content-Type: application/json; charset=UTF-8
X-Requested-With: XMLHttpRequest
{
  "opportunitySearch": {
    "Top": 20,
    "Skip": 0,
    "QueryString": "",
    "OrderBy": [
      {"Value": "postedDateDesc", "PropertyName": "PostedDate", "Ascending": false}
    ],
    "Filters": [
      {"t": "TermsSearchFilterDto", "fieldName": 5, "extra": null, "values": ["d3bcb1ec-cbcc-4681-8111-cbd37e2f7ae6"]}
    ]
  },
  "matchCriteria": {
    "PreferredJobs": [],
    "Educations": [],
    "LicenseAndCertifications": [],
    "Skills": [],
    "hasNoLicenses": false,
    "SkippedSkills": []
  }
}

HTTP 200
[Asserts]
jsonpath "$.opportunities[*].JobCategoryName" nth 0 == "Finance"
