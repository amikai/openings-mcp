package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/workday"
)

func main() {
	if err := workday.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
