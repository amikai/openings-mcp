package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/amazon"
)

func main() {
	if err := amazon.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
