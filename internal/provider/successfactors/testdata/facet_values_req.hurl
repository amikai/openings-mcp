POST https://jobs.sap.com/services/jobs/options/facetValues/
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json
Content-Type: application/json
{
  "q": "engineer",
  "locationsearch": "",
  "optionsFacetsDD_department": "",
  "optionsFacetsDD_customfield3": "",
  "optionsFacetsDD_country": ""
}

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.facets.map" exists
