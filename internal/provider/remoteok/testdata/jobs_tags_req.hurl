# Server-side tag filter: comma-separated values are AND-ed. The window
# is the ~100 most recent matches, so results can be older than the
# unfiltered dump's window.
GET https://remoteok.com/api
[QueryStringParams]
tags: golang

HTTP 200
[Asserts]
jsonpath "$[0].legal" exists
jsonpath "$[1].tags" contains "golang"
