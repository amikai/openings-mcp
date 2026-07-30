package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/nvidia"
)

func main() {
	if err := nvidia.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
