# Search jobs filtered by AreaID=A
GET https://rws.deltaww.com/OAGateWay/RWSV2/api/Index/SearchJobList?AreaID=A&AddJobName=&Lang=zh-TW

HTTP 200
[Asserts]
jsonpath "$" count > 50
jsonpath "$[0].AreaName" == "台灣"
