package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/successfactors"
)

func main() {
	if err := successfactors.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
