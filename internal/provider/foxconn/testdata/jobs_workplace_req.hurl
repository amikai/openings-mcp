# Happy-path list: a small server-side filtered result (workplaceCode=CH →
# 2 jobs of the full ~953-job board). No auth, cookies, or headers needed.
GET https://recruit.foxconn.com/hh_recruit_tw_api/portal_api/JobVacancies
[QueryStringParams]
workplaceCode: CH

HTTP 200
