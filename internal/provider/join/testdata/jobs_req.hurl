POST https://join.com/candidate-api/graphql
Host: join.com
content-type: application/json
accept: application/json
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
```graphql
query GetCompanyJobs {
  publicJobs(input: {companyId: 172617, paginationPage: {page: 1, pageSize: 100}}) {
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
jsonpath "$.data.publicJobs.items" count == 3
jsonpath "$.data.publicJobs.pageInfo.rowCount" == 3
