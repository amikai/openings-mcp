package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/remotive"
)

func main() {
	if err := remotive.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
