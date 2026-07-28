# moneyforward tenant, page 3. This was the short tail (203 = 100+100+3) when
# captured; as the total drifts this page may be a tail or already past the
# end, so only the pager — the client's stop condition — is asserted here.
# jobs_p3_rsp.html is the frozen 3-job tail the Go tests use for the stitch.
GET https://hrmos.co/pages/moneyforward/jobs?page=3

HTTP 200

[Asserts]
xpath "//nav[@class='pg-pagenation']" exists
