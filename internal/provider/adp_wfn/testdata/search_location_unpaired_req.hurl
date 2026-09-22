# The trap that forces validation before sending. "Clarinda" forms no
# value,qualifier pair, so the filter is dropped and the whole board comes
# back with HTTP 200 and an inflated total. Compare the count here with
# search_location_req.hurl, which filters the same city correctly.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
locationsList: Clarinda
[QueryStringParams]
cid: d16ba13d-474e-4326-b628-74c87f0b289a
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count == 20
jsonpath "$.meta.totalNumber" > 48
