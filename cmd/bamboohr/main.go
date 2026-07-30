package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/bamboohr"
)

func main() {
	if err := bamboohr.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
