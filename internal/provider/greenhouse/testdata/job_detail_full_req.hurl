# Retrieve a single job with every documented query parameter enabled (see
# ../openapi.yaml paths./boards/{board_token}/jobs/{job_id}.get).
GET https://boards-api.greenhouse.io/v1/boards/anthropic/jobs/4461450008
[QueryStringParams]
questions: true
pay_transparency: true

HTTP 200
