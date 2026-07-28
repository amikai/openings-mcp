# moneyforward tenant, page 1 of the full dump. 100 cassettes per full page;
# p.pg-count carries the true total (全 N 件中). The tenant-global facet nav
# (nav.sg-job-filters) carries 5 groups on this tenant: 雇用形態, 職種, 勤務地,
# 求人言語（Language）, 職種（詳細）.
#
# The exact total drifts as the employer posts and closes roles, so these
# assert the shape the parser depends on. The captured jobs_p1_rsp.html is
# the frozen 203-job snapshot the Go tests pin exact numbers against.
GET https://hrmos.co/pages/moneyforward/jobs

HTTP 200

[Asserts]
header "Content-Type" contains "text/html"
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count == 100
xpath "string(//p[@class='pg-count'])" matches "全\\s*\\d+\\s*件中"
xpath "//nav[@class='sg-job-filters']//th" count == 5
xpath "//nav[@class='pg-pagenation']" exists
