# Workplace filter enum: the reference list backing --workplace /
# workplaceCode. No auth, cookies, or headers needed.
GET https://recruit.foxconn.com/hh_recruit_tw_api/portal_api/Labels/Workplace/Codes

HTTP 200
[Asserts]
jsonpath "$" count > 0
jsonpath "$[0].code" exists
jsonpath "$[0].name" exists
