GET https://jobs.apple.com/api/v1/refData/teamsofinterest
Accept: application/json

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.res" count > 0
jsonpath "$.res[0].id" startsWith "teamsAndSubTeams-"
jsonpath "$.res[0].teams[0].code" exists
jsonpath "$.res[0].teams[0].teamCode" exists
jsonpath "$.res[0].teams[0].displayName" exists
