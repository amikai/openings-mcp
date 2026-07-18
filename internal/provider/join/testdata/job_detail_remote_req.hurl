# A REMOTE/ANYWHERE job: workplaceType REMOTE, remoteType ANYWHERE, but
# city/country still carry the employer's base location (Berlin, Germany)
# — captured to cover the case where a caller must not collapse Location
# to the bare city (see internal/ats/join.go's joinLocation).
GET https://join.com/companies/hey-contact-heroes/16433808-freelancer-m-w-d-gesucht-inbound-home-office-in-der-eu
Host: join.com
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36

HTTP 200

[Asserts]
header "Content-Type" contains "text/html"
xpath "//script[@id='__NEXT_DATA__']" count == 1
