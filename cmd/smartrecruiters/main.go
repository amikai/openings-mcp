package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/smartrecruiters"
)

func main() {
	if err := smartrecruiters.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
