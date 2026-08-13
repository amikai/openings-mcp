GET https://jobs.dayforcehcm.com/api/geo/pca/postingattributes/departments/pca/1/en-US
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Asserts]
jsonpath "$.postingAttributesAttributesList" count > 0
jsonpath "$.postingAttributesAttributesList[0].attributeId" isInteger

GET https://jobs.dayforcehcm.com/api/geo/pca/postingattributes/payclasses/pca/1/en-US
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Asserts]
jsonpath "$.postingAttributesAttributesList" count > 0

GET https://jobs.dayforcehcm.com/api/geo/pca/postingattributes/paytypes/pca/1/en-US
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36

HTTP 200
[Asserts]
jsonpath "$.postingAttributesAttributesList" count > 0
