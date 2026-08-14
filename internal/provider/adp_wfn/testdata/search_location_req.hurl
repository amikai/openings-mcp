# A published location pair filters server-side, but meta.totalNumber stops
# being a row count: it reports a relevance tally larger than the rows given.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
locationsList: Clarinda, IA
[QueryStringParams]
cid: d16ba13d-474e-4326-b628-74c87f0b289a
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count > 0
jsonpath "$.jobRequisitions[0].requisitionLocations[0].address.cityName" == "Clarinda"
