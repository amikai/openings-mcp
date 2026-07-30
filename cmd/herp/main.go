package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/herp"
)

func main() {
	if err := herp.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
