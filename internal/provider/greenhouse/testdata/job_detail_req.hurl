# Retrieve a single job, minimal shape (see
# ../openapi.yaml paths./boards/{board_token}/jobs/{job_id}.get). questions
# and pay_transparency are left false here; see job_detail_full_req.hurl for
# the fully-parameterized version.
GET https://boards-api.greenhouse.io/v1/boards/anthropic/jobs/4461450008
[QueryStringParams]
questions: false
pay_transparency: false

HTTP 200
