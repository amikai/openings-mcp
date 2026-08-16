# Job detail not found
GET https://rws.deltaww.com/OAGateWay/RWSV2/api/JobDetails/GetJobDetails?EmpAddID=INVALID123&Resumeid=&Lang=en-US

HTTP 200
[Asserts]
jsonpath "$.JobDetails" count == 0
