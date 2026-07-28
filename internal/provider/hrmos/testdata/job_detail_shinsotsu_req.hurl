# raksul/2167257001778999340: a new-graduate (新卒) posting from the merged
# sonar surface. Same page template, but with NO JobPosting JSON-LD — the
# parser falls back to the surrounding markup for these.
GET https://hrmos.co/pages/raksul/jobs/2167257001778999340

HTTP 200

[Asserts]
xpath "//script[@type='application/ld+json']" count == 0
xpath "//h1[@class='sg-corporate-name']" exists
xpath "//section[@class='pg-descriptions']" count == 2
xpath "//*[contains(@class,'pg-markdown')]" exists
