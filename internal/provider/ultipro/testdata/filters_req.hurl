POST https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/JobBoardView/GetFilters
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Content-Type: application/json; charset=UTF-8
X-Requested-With: XMLHttpRequest
{}

HTTP 200
[Asserts]
jsonpath "$.Filters" count == 4
