package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/ashby"
)

func main() {
	if err := ashby.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
