package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/nodesk"
)

func main() {
	if err := nodesk.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
