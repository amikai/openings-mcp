GET https://recruit.asus.com/Jobs/GetCities
[QueryStringParams]
countryTw: TW

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$" count >= 1
jsonpath "$[0].Value" == "TPE"
jsonpath "$[0].Text" == "臺北"
