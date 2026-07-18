# Salary floor + keyword + pagination combined. min0700 = first-year income
# (初年度年収) of at least 7,000,000 JPY; valid values are the fixed steps of
# the search form's srIncomeMin pulldown (0150-0550 in 50s, 0600, 0650,
# 0700, then 0800-1500 in 100s) — an off-step value like min0720 is HTTP 404.
GET https://tenshoku.mynavi.jp/list/min0700/kwPython/pg2/
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
accept-language: ja,en;q=0.9
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36

HTTP 200

[Asserts]
header "Content-Type" contains "text/html"
xpath "//p[@class='result__num']/em" exists
xpath "//div[contains(@class,'cassetteRecruit__content')]" count > 0
