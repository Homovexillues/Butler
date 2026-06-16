package cli

import (
	"context"
	"time"

	"butler/internal/engine"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "前台运行调度器",
	RunE: func(cmd *cobra.Command, args []string) error {
		nodes := []*model.Node{
			{
				Title: "通知测试",
				Schedule: schedule.Once{
					At: time.Now().Add(3 * time.Second),
				},
			},
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
