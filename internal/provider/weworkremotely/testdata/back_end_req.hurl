# Smallest live category feed — exercises the low-count end of the merge
# when Search fetches every category with no filter set.
GET https://weworkremotely.com/categories/remote-back-end-programming-jobs.rss
user-agent: Mozilla/5.0

HTTP 200

[Asserts]
header "Content-Type" contains "rss"
xpath "//item" count > 0
