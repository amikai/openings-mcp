package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/engage"
)

func main() {
	if err := engage.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
