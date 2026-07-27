# moneyforward tenant, page 1 of the full 203-job dump. 100 cassettes per
# full page; p.pg-count carries the true total (全 N 件中). The tenant-global
# facet nav (nav.sg-job-filters) carries 5 groups on this tenant: 雇用形態,
# 職種, 勤務地, 求人言語（Language）, 職種（詳細）.
GET https://hrmos.co/pages/moneyforward/jobs

HTTP 200

[Asserts]
header "Content-Type" contains "text/html"
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count == 100
xpath "string(//p[@class='pg-count'])" contains "全 203 件"
xpath "//nav[@class='sg-job-filters']//th" count == 5
xpath "//nav[@class='pg-pagenation']" exists
