package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/workable"
)

func main() {
	if err := workable.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
