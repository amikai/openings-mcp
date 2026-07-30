package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/lever"
)

func main() {
	if err := lever.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
