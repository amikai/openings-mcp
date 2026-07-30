package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/synopsys"
)

func main() {
	if err := synopsys.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
