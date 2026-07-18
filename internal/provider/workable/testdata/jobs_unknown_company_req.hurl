# An unknown account 404s with a text/plain body ("Not Found"), unlike
# SmartRecruiters where an unknown company is a 200 with an empty page.
POST https://apply.workable.com/api/v3/accounts/openings-mcp-no-such-account/jobs
Content-Type: application/json
{
  "query": ""
}

HTTP 404
