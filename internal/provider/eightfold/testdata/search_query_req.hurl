# Proves query and location are real server-side filters, not client-side
# echoes: count narrows from 1330 to 43, and every returned title contains
# "engineer" while every location contains "New York".
GET https://morganstanley.eightfold.ai/api/pcsx/search
[QueryStringParams]
domain: morganstanley.com
query: engineer
location: New York
start: 0

HTTP 200
