# Search jobs empty result
GET https://rws.deltaww.com/OAGateWay/RWSV2/api/Index/SearchJobList?AreaID=&AddJobName=NONEXISTENT_KEYWORD_XYZ&Lang=en-US

HTTP 200
[Asserts]
jsonpath "$" count == 0
