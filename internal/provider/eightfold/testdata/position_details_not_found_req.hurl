# Unlike SmartRecruiters' RESOURCE_NOT_FOUND envelope, Eightfold's 404 body
# reuses the success envelope shape with an empty `data: {}` object.
GET https://morganstanley.eightfold.ai/api/pcsx/position_details
[QueryStringParams]
position_id: 1
domain: morganstanley.com
hl: en

HTTP 404
