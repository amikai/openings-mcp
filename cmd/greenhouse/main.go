package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/greenhouse"
)

func main() {
	if err := greenhouse.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
