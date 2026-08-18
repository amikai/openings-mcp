# freehire

`openapi.yaml` is https://freehire.me/openapi.yaml, used unmodified. Two
limits it does not state, both measured against the live API and pinned
by `testdata/*.hurl`:

- Paging stops at `MaxResultWindow` rows; past it the API answers 400
  `pagination too deep`.
- `GET /agent/jobs/search` returns every row's full description whatever
  `description_format` asks for, and `GET /jobs/{slug}` takes no such
  parameter, so detail is always the stored HTML.
