package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/workingnomads"
)

func main() {
	if err := workingnomads.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
