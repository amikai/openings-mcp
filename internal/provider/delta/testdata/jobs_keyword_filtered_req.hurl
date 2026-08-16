# Search jobs filtered by keyword
GET https://rws.deltaww.com/OAGateWay/RWSV2/api/Index/SearchJobList?AreaID=&AddJobName=%E8%BB%9F%E9%AB%94&Lang=zh-TW

HTTP 200
[Asserts]
jsonpath "$" count >= 1
jsonpath "$[0].JobName" contains "軟體"
