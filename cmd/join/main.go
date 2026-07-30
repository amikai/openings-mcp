package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/join"
)

func main() {
	if err := join.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
