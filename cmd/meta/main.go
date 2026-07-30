package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/meta"
)

func main() {
	if err := meta.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
