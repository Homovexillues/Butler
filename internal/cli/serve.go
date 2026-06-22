package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"butler/internal/config"
	"butler/internal/engine"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "前台运行调度器",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("fail to load config:\n%w", err)
		}

		nodes, err := config.LoadPlan()
		if err != nil {
			return fmt.Errorf("fail to load plan:\n%w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		registry := buildRegistry(cfg)

		engine.Run(ctx, registry, nodes)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
