# Tech Stack

- Go 1.26.4 module.
- MCP server/client integration uses `github.com/modelcontextprotocol/go-sdk/mcp` v1.6.1.
- Tests use Go `testing`; `github.com/stretchr/testify` is available but not required everywhere.
- OpenAPI-generated Cake provider code uses `github.com/ogen-go/ogen`; OpenAPI validation target depends on Docker image `pythonopenapi/openapi-spec-validator`.
- Legacy/prototype subprojects exist (`104-mcp-server` TypeScript/Vercel, `job104-mcp` Python), but the primary Go server lives at repo root.