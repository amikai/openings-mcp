package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/mokahr"
)

func main() {
	if err := mokahr.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
