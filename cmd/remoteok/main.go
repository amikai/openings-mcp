package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/remoteok"
)

func main() {
	if err := remoteok.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
