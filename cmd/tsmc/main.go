package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/tsmc"
)

func main() {
	if err := tsmc.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
