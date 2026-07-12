# Proves q= is a real server-side keyword filter, not a client-side echo:
# totalFound narrows to 138 (of Equinox's ~662 postings) and every returned
# title contains "trainer".
GET https://api.smartrecruiters.com/v1/companies/equinox/postings
[QueryStringParams]
q: trainer
limit: 3

HTTP 200
