# Keep content=false so the response matches jobs_rsp.json.
GET https://boards-api.greenhouse.io/v1/boards/safariai/jobs
[QueryStringParams]
content: false

HTTP 200
