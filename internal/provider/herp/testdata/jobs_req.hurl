# Paginates by COMPANY, not by job: total (2198) and totalPages count
# companies, and each row carries at most two preview jobs however many that
# company really has. limit caps at 100 — limit=200 is a 400. The
# filter-looking params the web UI puts in its URLs (employment-type-id,
# parent-job-role-ids, remote-work-type-ids) are no-ops here: the response is
# byte-identical with and without them.
GET https://herp.careers/careers/api/v1/jobs
[QueryStringParams]
page: 1
limit: 2

HTTP 200
