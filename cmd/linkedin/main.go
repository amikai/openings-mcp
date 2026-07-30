package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/linkedin"
)

func main() {
	if err := linkedin.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
