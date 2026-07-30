package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/teamtailor"
)

func main() {
	if err := teamtailor.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
