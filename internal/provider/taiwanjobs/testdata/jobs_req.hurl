# TaiwanJobs open feed: two rows, no filters.
GET https://free.taiwanjobs.gov.tw/webservice_taipei/Webservice.ashx?count=2
User-Agent: Mozilla/5.0 (compatible; openings-mcp/taiwanjobs)
Accept: text/xml,application/xml;q=0.9,*/*;q=0.8
HTTP 200
[Asserts]
body contains "<DataList>"
body contains "OCCU_DESC"
body contains "COMPNAME"
body contains "URL_QUERY"
