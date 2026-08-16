# MediaTek Careers Provider — Plan

Design: [specs/2026-07-20-mediatek-provider-design.md](../specs/2026-07-20-mediatek-provider-design.md)

1. ~~Spec hunt/recon~~ — confirmed no official API spec; recovered and
   replayed the public tRPC search endpoint and SSR detail route.
2. ~~Fixtures~~ — captured real filtered, keyword, empty, detail, and
   unknown-detail responses.
3. Provider package — add the hand-written client, HTML/JSON parsers, mock
   server, and fixture-replaying tests.
4. Debug CLI — add `search` and `detail` commands with live manual checks.
5. MCP surface — intentionally deferred by request; do not modify
   `internal/openingsmcp` or `cmd/openings-mcp`.
