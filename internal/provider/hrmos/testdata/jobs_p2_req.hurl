# moneyforward tenant, page 2: a second page reached via ?page=N. How many
# cassettes it holds depends on the tenant's current total, so this asserts
# only that the page paginates and still reports a total.
GET https://hrmos.co/pages/moneyforward/jobs?page=2

HTTP 200

[Asserts]
xpath "//li[@class='pg-list-cassette jsc-joblist-cassette']" count >= 1
xpath "string(//p[@class='pg-count'])" matches "全\\s*\\d+\\s*件中"
