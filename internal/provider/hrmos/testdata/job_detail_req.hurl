# moneyforward/0000265: a posting whose salary is 応相談 (negotiable), so the
# JSON-LD baseSalary key is null.
GET https://hrmos.co/pages/moneyforward/jobs/0000265

HTTP 200

[Asserts]
xpath "//script[@type='application/ld+json']" count == 1
xpath "string(//script[@type='application/ld+json'])" contains "\"@type\":\"JobPosting\""
xpath "string(//script[@type='application/ld+json'])" contains "\"baseSalary\":null"
xpath "//section[@class='pg-descriptions']" count == 2
