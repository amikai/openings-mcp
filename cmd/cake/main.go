package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/cake"
)

func main() {
	if err := cake.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
