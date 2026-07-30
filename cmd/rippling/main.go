package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/rippling"
)

func main() {
	if err := rippling.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
