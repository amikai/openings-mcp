# visional/0000381: a posting with a populated baseSalary
# (min 6000000 / max 10000000 / JPY / YEAR), the other half of fact 15.
GET https://hrmos.co/pages/visional/jobs/0000381

HTTP 200

[Asserts]
xpath "//script[@type='application/ld+json']" count == 1
xpath "string(//script[@type='application/ld+json'])" contains "\"minValue\":6000000"
xpath "string(//script[@type='application/ld+json'])" contains "\"maxValue\":10000000"
xpath "string(//script[@type='application/ld+json'])" contains "\"unitText\":\"YEAR\""
