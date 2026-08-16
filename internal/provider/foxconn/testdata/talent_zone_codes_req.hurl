# Talent-zone filter enum: the reference list backing --talent-zone /
# talentZoneCode. Entries include dated recruitment-campaign zones
# (e.g. Y221209-01), so this list turns over more than the workplace one.
GET https://recruit.foxconn.com/hh_recruit_tw_api/portal_api/Labels/TalentZone/Codes

HTTP 200
[Asserts]
jsonpath "$" count > 0
jsonpath "$[0].code" exists
jsonpath "$[0].name" exists
