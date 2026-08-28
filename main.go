package main

import (
	"log/slog"
	"os"

	"butler/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		slog.Error("fail to execute butler command", "error", err)
		os.Exit(1)
	}
}
