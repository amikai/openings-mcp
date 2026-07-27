# An unknown slug 404s with a constant, message-less body
# ({"error":{"message":"Function"}}), so the adapter supplies its own text.
GET https://herp.careers/careers/api/v1/companies/this-company-does-not-exist-xyz

HTTP 404
