package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/foxconn"
)

func main() {
	if err := foxconn.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
