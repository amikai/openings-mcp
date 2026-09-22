# Read for one field only: the repeated Locale entries, whose first value is
# the tenant's job-bearing locale. There is no company name anywhere in this
# response — the career-center header is a logo image.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/content-links/career-center
[QueryStringParams]
cid: 5a312740-8c7b-44b7-80ad-50a35fbb25d3
locale: en_CA

HTTP 200
[Asserts]
jsonpath "$.meta.customFieldGroup.stringFields[?(@.nameCode.codeValue=='Locale')].stringValue" exists
