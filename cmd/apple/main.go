package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/apple"
)

func main() {
	if err := apple.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
