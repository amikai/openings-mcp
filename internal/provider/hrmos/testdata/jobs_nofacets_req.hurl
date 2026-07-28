# hrmos tenant (株式会社ビズリーチ): 86 jobs, single page, and no facet nav at
# all (fact 4) — tenants can opt out of nav.sg-job-filters entirely.
GET https://hrmos.co/pages/hrmos/jobs

HTTP 200

[Asserts]
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count == 86
xpath "string(//p[@class='pg-count'])" contains "全 86 件"
xpath "//nav[@class='sg-job-filters']" count == 0
