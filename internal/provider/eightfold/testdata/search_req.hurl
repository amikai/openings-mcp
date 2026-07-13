# Captured 2026-07-14 from morganstanley.eightfold.ai. Unfiltered first page:
# count (1330) is the tenant's total open positions, independent of the 10
# returned here — pagination is a fixed page size of 10 via start=<offset>.
GET https://morganstanley.eightfold.ai/api/pcsx/search
[QueryStringParams]
domain: morganstanley.com
query:
location:
start: 0

HTTP 200
