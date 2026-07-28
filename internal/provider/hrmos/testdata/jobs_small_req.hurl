# visional tenant: a single-page (9-job) dump with a different facet-group
# vocabulary (雇用形態, 求人カテゴリー) than moneyforward.
GET https://hrmos.co/pages/visional/jobs

HTTP 200

[Asserts]
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count == 9
xpath "string(//p[@class='pg-count'])" contains "全 9 件"
xpath "//nav[@class='sg-job-filters']//th" count == 2
