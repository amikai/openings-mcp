package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/jobindex"
)

func main() {
	if err := jobindex.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
