package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/oracle"
)

func main() {
	if err := oracle.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
