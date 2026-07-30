package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/eightfold"
)

func main() {
	if err := eightfold.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
