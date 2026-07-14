POST https://apis.indeed.com/graphql
Host: apis.indeed.com
content-type: application/json
indeed-api-key: 161092c2017b5bbab13edb12461a62d5a833871e7cad6d9d475304573de67ac8
accept: application/json
indeed-locale: en-US
user-agent: Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Indeed App 193.1
indeed-app-info: appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone
indeed-co: TW
```graphql
query GetJobDetail {
  jobData(jobKeys: ["9d503ca7fe211430"]) {
    results {
      job {
        key
        title
        datePublished
        dateOnIndeed
        description { html }
        location { countryName countryCode admin1Code city postalCode streetAddress formatted { short long } }
        compensation {
          estimated { currencyCode baseSalary { unitOfWork range { ... on Range { min max } } } }
          baseSalary { unitOfWork range { ... on Range { min max } } }
          currencyCode
        }
        attributes { key label }
        employer {
          relativeCompanyPageUrl
          name
          dossier {
            employerDetails { addresses industry employeesLocalizedLabel revenueLocalizedLabel briefDescription ceoName ceoPhotoUrl }
            images { headerImageUrl squareLogoUrl }
            links { corporateWebsite }
          }
        }
        recruit { viewJobUrl detailedSalary workSchedule }
        source { name }
      }
    }
  }
}
```

HTTP 200

[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.data.jobData.results[0].job.key" == "9d503ca7fe211430"
