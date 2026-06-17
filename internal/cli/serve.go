package cli

import (
	"context"
	"log"

	"butler/internal/config"
	"butler/internal/engine"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/parser"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "前台运行调度器",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := config.Load()
		if err != nil {
			log.Fatalf("Fail to load config:%s", err.Error())
		}
		nodes, err := parser.Parse[[]*model.Node](config.PlanPath)
		if err != nil {
			log.Fatalf("Fail to parse plan:%s", err.Error())
		}
		ctx := context.Background()
		notifier := notify.SystemNotifier{}
		engine.Run(ctx, nodes, notifier)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
