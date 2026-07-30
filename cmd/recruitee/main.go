package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/recruitee"
)

func main() {
	if err := recruitee.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
