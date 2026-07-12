# Quirk: an unknown companyIdentifier is NOT a 404 here, unlike Greenhouse's
# board endpoint. It's HTTP 200 with the same empty shape a real company
# with zero open postings would return (totalFound: 0, content: []).
GET https://api.smartrecruiters.com/v1/companies/this-company-does-not-exist-xyz/postings

HTTP 200
