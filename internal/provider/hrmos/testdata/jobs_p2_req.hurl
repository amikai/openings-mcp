# moneyforward tenant, page 2 of the full 203-job dump: a second full page.
GET https://hrmos.co/pages/moneyforward/jobs?page=2

HTTP 200

[Asserts]
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count == 100
xpath "string(//p[@class='pg-count'])" contains "全 203 件"
