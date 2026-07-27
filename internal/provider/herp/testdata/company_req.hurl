# One tenant's whole board in a single response: full JD text, salary, and
# the company profile all arrive here, and there is no per-job endpoint —
# /careers/api/v1/companies/{slug}/jobs/{id} 404s.
GET https://herp.careers/careers/api/v1/companies/notainc

HTTP 200
