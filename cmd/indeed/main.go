package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/indeed"
)

func main() {
	if err := indeed.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
