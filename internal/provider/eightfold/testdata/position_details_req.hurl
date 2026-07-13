# position_id here matches search_rsp.json's first position, so client_test.go
# can chain a search result straight into a detail fetch.
GET https://morganstanley.eightfold.ai/api/pcsx/position_details
[QueryStringParams]
position_id: 549798858854
domain: morganstanley.com
hl: en

HTTP 200
