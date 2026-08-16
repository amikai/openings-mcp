# Fetch area list reference data
GET https://rws.deltaww.com/OAGateWay/RWSV2/api/Index/GetAreaList?Lang=en-US

HTTP 200
[Asserts]
jsonpath "$[0].Value" == "A"
jsonpath "$[0].Text" == "TW"
