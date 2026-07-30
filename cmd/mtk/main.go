package main

import (
	"os"

	"github.com/amikai/openings-mcp/internal/cli/provider/mtk"
)

func main() {
	if err := mtk.NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
