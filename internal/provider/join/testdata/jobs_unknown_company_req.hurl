# A companyId that was never allocated. The API returns HTTP 200 with an
# empty result set rather than an error or 404 — there is no way to
# distinguish this from a real company with zero jobs (see
# jobs_empty_req.hurl) from this call alone.
POST https://join.com/candidate-api/graphql
Host: join.com
content-type: application/json
accept: application/json
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
```graphql
query GetCompanyJobs {
  publicJobs(input: {companyId: 999999999, paginationPage: {page: 1, pageSize: 100}}) {
    items {
      id
      idParam
      title
      status
      workplaceType
      remoteType
      createdAt
      updatedAt
      city { cityName countryName }
      country { iso3166 name }
      category { id name }
      employmentType { id name }
    }
    pageInfo { page pageCount pageSize rowCount }
  }
}
```

HTTP 200

[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.data.publicJobs.items" count == 0
