# Unknown category slug: no such feed exists. The site answers with a 301
# that carries no Location header (a Rails routing quirk, not a real
# redirect target) rather than a clean 404 — the client never sends this
# request because Category is validated against the known set first.
GET https://weworkremotely.com/categories/remote-bogus-category-jobs.rss
user-agent: Mozilla/5.0

HTTP 301
