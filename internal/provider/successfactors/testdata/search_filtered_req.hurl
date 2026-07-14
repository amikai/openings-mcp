GET https://jobs.sap.com/search/
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-US,en;q=0.9
[QueryStringParams]
q: engineer
locationsearch:
optionsFacetsDD_department: Software-Design and Development
optionsFacetsDD_customfield3:
optionsFacetsDD_country: DE
startrow: 0

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "//html" exists
