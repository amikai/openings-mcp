# Detail returns exactly the list-item shape — no field a search doesn't
# already carry. Captured 2026-07-31; this slug 404s once the job is taken
# down, at which point recapture against any live slug with a salary.
GET https://4dayweek.io/api/v2/jobs/senior-infrastructure-engineer-at-buffer-38679a55

HTTP 200
[Asserts]
jsonpath "$.slug" == "senior-infrastructure-engineer-at-buffer-38679a55"
jsonpath "$.work_arrangement" == "remote"
# Fully remote with no office: the locations array is absent, not empty.
jsonpath "$.locations" not exists
# Salary is in cents, unlike the whole-dollar salary_min/salary_max filters.
jsonpath "$.salary_min" > 1000000
jsonpath "$.company.slug" == "buffer"
