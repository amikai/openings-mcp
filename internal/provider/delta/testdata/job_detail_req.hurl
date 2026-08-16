# Job detail happy path
GET https://rws.deltaww.com/OAGateWay/RWSV2/api/JobDetails/GetJobDetails?EmpAddID=C20260814001&Resumeid=&Lang=en-US

HTTP 200
[Asserts]
jsonpath "$.JobDetails[0].EmpAddID" == "C20260814001"
jsonpath "$.JobDetails[0].JobName" == "自動化資深工程師"
