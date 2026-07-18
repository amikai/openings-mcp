# An unknown tag is not an error: still HTTP 200, with only the legal
# element in the array.
GET https://remoteok.com/api
[QueryStringParams]
tags: doesnotexisttag12345

HTTP 200
[Asserts]
jsonpath "$[0].legal" exists
jsonpath "$" count == 1
