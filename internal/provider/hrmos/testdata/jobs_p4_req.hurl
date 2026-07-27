# moneyforward tenant, page 4: past the end of the 3-page dump. HTTP 200
# with zero cassettes and no p.pg-count/section#jsi-joblist — but the pager
# nav is still present and still lists the real 1..3 page range, which is
# what the client uses as the reliable stop condition (fact 14).
GET https://hrmos.co/pages/moneyforward/jobs?page=4

HTTP 200

[Asserts]
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count == 0
xpath "//p[@class='pg-count']" count == 0
xpath "//nav[@class='pg-pagenation']" exists
xpath "//nav[@class='pg-pagenation']//a" count == 3
