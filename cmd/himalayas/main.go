package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/himalayas"
)

func main() {
	if err := himalayas.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
