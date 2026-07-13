# Facet filters are undocumented anywhere public; this exact param shape
# (filter_<facetName>=<value>, repeatable for OR) was recovered by
# instrumenting fetch/XHR in a live browser session and clicking the
# "Business Area: Technology" checkbox, not by reading any spec. count
# narrows from 1330 to 112 and appliedFilters.businessarea echoes the value.
GET https://morganstanley.eightfold.ai/api/pcsx/search
[QueryStringParams]
domain: morganstanley.com
query:
location:
start: 0
filter_businessarea: technology

HTTP 200
