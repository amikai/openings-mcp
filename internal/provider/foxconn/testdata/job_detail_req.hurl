# Detail happy path, keyed by the opaque `id` (not job_no). job_type and
# job_type_name come back null here though they are populated on the list
# endpoint for this same job.
GET https://recruit.foxconn.com/hh_recruit_tw_api/portal_api/JobVacancies/08de75d7bd7611a790b20d5b47c2f1bb

HTTP 200
[Asserts]
jsonpath "$.job_no" == "PP26022700001"
jsonpath "$.job_type" == null
