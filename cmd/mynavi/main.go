package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/mynavi"
)

func main() {
	if err := mynavi.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
