package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/hrmos"
)

func main() {
	if err := hrmos.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
