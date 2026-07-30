// Command verify-companies verifies every curated companies.yaml entry by
// running a real search through the unified internal/ats adapters — the
// same code path the MCP server serves — and reports each entry's total
// job count. Each successful search is followed by one Detail probe on a
// sampled job, so detail-template divergence surfaces here rather than at
// release smoke testing. See
// docs/superpowers/specs/2026-07-12-verify-companies-cmd-design.md.
package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/verifycompanies"
)

func main() {
	if err := verifycompanies.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
