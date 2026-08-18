# freehire

`openapi.yaml` is https://freehire.me/openapi.yaml, used unmodified.

Two things a caller has to know before trusting a filtered result, and
several places the spec and the live API disagree, are written up in
`doc.go`. `testdata/*.hurl` pins each of them against the live API —
`jobs_ignored_params_req.hurl` and `jobs_deep_page_req.hurl` in
particular cover behaviour `openapi.yaml` states in prose only.
