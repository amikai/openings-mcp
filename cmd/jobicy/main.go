package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/jobicy"
)

func main() {
	if err := jobicy.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
