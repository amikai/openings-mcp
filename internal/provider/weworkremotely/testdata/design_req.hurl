# Filtered search: category maps 1:1 to its own feed URL, so filtering by
# category means fetching only that one feed instead of every category.
GET https://weworkremotely.com/categories/remote-design-jobs.rss
user-agent: Mozilla/5.0

HTTP 200

[Asserts]
header "Content-Type" contains "rss"
xpath "//item" count > 0
