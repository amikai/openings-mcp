package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/ultipro"
)

func main() {
	if err := ultipro.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
