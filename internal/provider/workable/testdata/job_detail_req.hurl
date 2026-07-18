# The posting body is split across three HTML fields: description,
# requirements, and benefits. This posting was published 2026-07-14; when it
# closes this request starts to 404 and the fixture needs recapturing.
GET https://apply.workable.com/api/v2/accounts/blueground/jobs/B02DA69C8F

HTTP 200
[Asserts]
jsonpath "$.description" exists
