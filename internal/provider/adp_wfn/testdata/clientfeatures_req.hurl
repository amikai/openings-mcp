# The only public field carrying a tenant's company name. Note the path: this
# operation sits outside the /v1 prefix every other one uses. cid alone is
# enough.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/client-features
[QueryStringParams]
cid: d16ba13d-474e-4326-b628-74c87f0b289a

HTTP 200
[Asserts]
jsonpath "$.meta.customFieldGroup.stringFields[?(@.nameCode.codeValue=='ClientName')].stringValue" exists
jsonpath "$.meta.customFieldGroup.stringFields[?(@.nameCode.codeValue=='ClientID')].stringValue" exists
