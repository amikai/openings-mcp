# Captures Ashby public job-board fixtures from real boards, exercising both
# states of the only documented query parameter, includeCompensation (see
# ../openapi.yaml paths./posting-api/job-board/{jobBoardName}.get):
# - `browserbase` (small - 5 jobs at capture time - but exercises
#   secondaryLocations, streetAddress, compensation tiers, and null tier
#   titles)
# - `weaviate` (4 jobs at capture time; its jobs carry null isRemote and
#   null workplaceType, which the official docs claim are always present)
GET https://api.ashbyhq.com/posting-api/job-board/browserbase

HTTP 200

GET https://api.ashbyhq.com/posting-api/job-board/browserbase
[QueryStringParams]
includeCompensation: true

HTTP 200

GET https://api.ashbyhq.com/posting-api/job-board/weaviate
[QueryStringParams]
includeCompensation: true

HTTP 200
