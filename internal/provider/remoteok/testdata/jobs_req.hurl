# Full dump: the ~100 most recent jobs. Element 0 is always the
# legal/attribution notice, not a job.
GET https://remoteok.com/api

HTTP 200
[Asserts]
jsonpath "$[0].legal" exists
jsonpath "$[1].id" exists
jsonpath "$[1].slug" exists
