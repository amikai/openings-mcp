# Keyword search, page 1 of the national /list/ surface. 50 cards per full
# page; the total lives in p.result__num. Path segments are the query DSL:
# kw{keywords} (space-separated terms AND together), min{NNNN} (annual-salary
# floor in units of 10,000 JPY, fixed steps only), pg{N} (1-based page). Segment order is
# free; unknown segments 404 — except unknown *prefix* paths like
# /tokyo/list/, which soft-fall back to an unfiltered legacy Shift_JIS page,
# so only build URLs from verified tokens.
GET https://tenshoku.mynavi.jp/list/kwPython/
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
accept-language: ja,en;q=0.9
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36

HTTP 200

[Asserts]
header "Content-Type" contains "text/html"
xpath "//p[@class='result__num']/em" exists
xpath "//div[contains(@class,'cassetteRecruit__content')]" count == 50
xpath "//p[contains(@class,'cassetteRecruit__copy')]//a[contains(@href,'jobinfo-')]" exists
