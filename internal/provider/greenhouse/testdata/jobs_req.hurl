# List all jobs for a board (see ../openapi.yaml paths./boards/{board_token}/jobs.get).
# content is left false here so the response matches jobs_rsp.json's shape;
# see jobs_content_rsp.json for the content=true shape (hand-crafted, not a
# live capture).
GET https://boards-api.greenhouse.io/v1/boards/safariai/jobs
[QueryStringParams]
content: false

HTTP 200
