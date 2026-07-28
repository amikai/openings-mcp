# moneyforward tenant, page 4: past the end of the dump. HTTP 200 with zero
# cassettes and no p.pg-count/section#jsi-joblist — but the pager nav is still
# present and still lists the real page range, which is what the client uses
# as its reliable stop condition (fact 14). The pager's link count tracks the
# tenant's current size, so it is not asserted.
GET https://hrmos.co/pages/moneyforward/jobs?page=4

HTTP 200

[Asserts]
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count == 0
xpath "//p[@class='pg-count']" count == 0
xpath "//nav[@class='pg-pagenation']" exists
