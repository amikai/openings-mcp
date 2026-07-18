# Facets for the search body's filter fields. "department" filtering wants the
# numeric ids listed here, not names; locations are matched by object fields
# (country/region/city), any subset of which may be sent.
GET https://apply.workable.com/api/v3/accounts/blueground/jobs/filters

HTTP 200
[Asserts]
jsonpath "$.locations" exists
jsonpath "$.workplaces" exists
