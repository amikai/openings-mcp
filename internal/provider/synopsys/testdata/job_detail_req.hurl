# City and slug are cosmetic; company_id and job_id identify the posting.
GET https://careers.synopsys.com/job/bengaluru/staff-software-engineer/44408/93498496944

HTTP 200

[Asserts]
header "Content-Type" contains "text/html"
xpath "//html" exists
