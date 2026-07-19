# Happy path: one category feed, full item set (no server-side paging or
# filtering — this IS the dump for the Full-Stack Programming category).
GET https://weworkremotely.com/categories/remote-full-stack-programming-jobs.rss
user-agent: Mozilla/5.0

HTTP 200

[Asserts]
header "Content-Type" contains "rss"
xpath "//item" count > 0
body contains "<title>We Work Remotely"
