# Search jobs happy path (unfiltered)
GET https://rws.deltaww.com/OAGateWay/RWSV2/api/Index/SearchJobList?AreaID=&AddJobName=&Lang=en-US

HTTP 200
[Asserts]
jsonpath "$" count > 100
jsonpath "$[0].EmpAddID" exists
jsonpath "$[0].JobName" exists
