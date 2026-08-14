# The safe failure: a well-formed pair naming a place the board does not use
# returns zero rows rather than the whole board.
GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions
locationsList: Atlantis,LOCATION_CITY
[QueryStringParams]
cid: d16ba13d-474e-4326-b628-74c87f0b289a
locale: en_US

HTTP 200
[Asserts]
jsonpath "$.jobRequisitions" count == 0
