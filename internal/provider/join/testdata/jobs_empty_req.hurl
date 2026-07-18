# bsventure (Firstlane Capital GmbH) — a confirmed real join.com company
# with zero open positions right now, distinct from an unknown/invalid
# company id (see jobs_unknown_company_req.hurl): both return the same
# empty-items shape since the API has no dedicated not-found response.
POST https://join.com/candidate-api/graphql
Host: join.com
content-type: application/json
accept: application/json
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
```graphql
query GetCompanyJobs {
  publicJobs(input: {companyId: 154838, paginationPage: {page: 1, pageSize: 100}}) {
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
jsonpath "$.data.publicJobs.pageInfo.rowCount" == 0
