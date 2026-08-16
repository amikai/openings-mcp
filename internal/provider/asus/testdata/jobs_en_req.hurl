# The category filter submits the category's localized label as its value, so
# which values filter depends on the session's locale. Pick en-US, then read
# the same form back.
GET https://recruit.asus.com/Home/SetLanguage?culture=en-US&returnUrl=~%2FJobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
[Options]
location: false

HTTP 302

GET https://recruit.asus.com/Jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Captures]
unfiltered_pages: xpath "string(//li[contains(@class, 'PagedList-skipToLast')]/a/@href)" regex "page=([0-9]+)" toInt
[Asserts]
xpath "//input[@name='REQ_TYPEs_Prefix']/@value" count == 4
xpath "string((//input[@name='REQ_TYPEs_Prefix']/@value)[1])" == "Research and Development"
# Country and experience values stay locale-independent codes; only their
# labels translate.
xpath "string(//select[@id='Location']/option[@value='TW'])" == "Taiwan"
xpath "string(//select[@id='WORK_EXP']/option[@value='0'])" == "Less than 2 Years"

# The locale's own value filters.
GET https://recruit.asus.com/Jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
[QueryStringParams]
REQ_TYPEs_Prefix: Research and Development

HTTP 200
[Asserts]
xpath "string(//li[contains(@class, 'PagedList-skipToLast')]/a/@href)" regex "page=([0-9]+)" toInt < {{unfiltered_pages}}

# The other locale's value is not rejected, it is ignored: the board comes back
# whole under HTTP 200, so a stale value reads as "this category has every job"
# rather than as an error.
GET https://recruit.asus.com/Jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
[QueryStringParams]
REQ_TYPEs_Prefix: 研究發展

HTTP 200
[Asserts]
xpath "string(//li[contains(@class, 'PagedList-skipToLast')]/a/@href)" regex "page=([0-9]+)" toInt == {{unfiltered_pages}}
