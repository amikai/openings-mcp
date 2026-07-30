package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/job104"
)

func main() {
	if err := job104.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
