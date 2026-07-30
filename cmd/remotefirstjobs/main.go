package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/remotefirstjobs"
)

func main() {
	if err := remotefirstjobs.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
