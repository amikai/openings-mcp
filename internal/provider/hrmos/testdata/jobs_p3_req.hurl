# moneyforward tenant, page 3: the short tail (203 - 100 - 100 = 3 jobs).
GET https://hrmos.co/pages/moneyforward/jobs?page=3

HTTP 200

[Asserts]
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count == 3
xpath "string(//p[@class='pg-count'])" contains "全 203 件"
