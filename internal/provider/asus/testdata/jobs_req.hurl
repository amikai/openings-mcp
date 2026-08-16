GET https://recruit.asus.com/Jobs
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
xpath "//table[contains(@class, 'Rwd-Table')]" exists
# The search form is the only place the category, country and experience
# filter values are published, so the client reads them from here.
xpath "//input[@name='REQ_TYPEs_Prefix']/@value" count == 4
xpath "string((//input[@name='REQ_TYPEs_Prefix']/@value)[1])" == "研究發展"
xpath "//select[@id='Location']/option[@value!='']/@value" count >= 80
xpath "string(//select[@id='Location']/option[@value='TW'])" == "臺灣"
# Slovenia deviates from ISO 3166-1 alpha-2 (SI).
xpath "//select[@id='Location']/option[@value='SL']" exists
xpath "//select[@id='WORK_EXP']/option[@value!='']/@value" count == 5
# The city select ships empty; /Jobs/GetCities fills it once a country is picked.
xpath "//select[@id='City']/option[@value!='']" not exists
