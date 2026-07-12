# department.id is an unquoted integer here (like the posting detail
# endpoint, unlike the posting list endpoint), and entries carry an
# `archived` boolean the official document omits.
GET https://api.smartrecruiters.com/v1/companies/equinox/departments

HTTP 200
