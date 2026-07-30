package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/quanta"
)

func main() {
	if err := quanta.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
