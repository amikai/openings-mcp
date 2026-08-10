# Proves work_arrangement/country/schedule are real server-side filters:
# total narrows from the full board (22207 when captured) to 3.
GET https://4dayweek.io/api/v2/jobs
[QueryStringParams]
work_arrangement: remote
country: Germany
schedule: 4_day_week
limit: 2

HTTP 200
[Asserts]
jsonpath "$.total" < 100
jsonpath "$.data[0].work_arrangement" == "remote"
jsonpath "$.data[0].schedule_type" == "4_day_week"
# country matches any location, ignoring that location's own arrangement, so
# a match does not imply the job is remote-workable from that country.
jsonpath "$.data[*].locations[*].country" contains "Germany"
