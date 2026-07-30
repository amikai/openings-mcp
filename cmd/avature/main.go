package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/avature"
)

func main() {
	if err := avature.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
