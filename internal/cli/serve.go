package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"butler/internal/config"
	"butler/internal/engine"
	"butler/internal/model"
	"butler/internal/schedule"

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
		cronSchedule, err := schedule.NewCronSchedule("*/1 * * * *")
		if err != nil {
			return fmt.Errorf("fail to make new CronSchedule:\n%w", err)
		}
		nodes := []*model.Node{
			{
				Title: "test once message",
				Schedule: schedule.Once{
					At: time.Now().Add(3 * time.Second),
				},
				Channels: []string{"system", "mqtt", "email"},
			}, {
				Title:    "test cron message",
				Schedule: cronSchedule,
				Channels: []string{"system", "mqtt"},
			}, {
				Title: "test solar message",
				Schedule: schedule.SolarAnnual{
					Month:  6,
					Day:    18,
					Hour:   11,
					Minute: 14,
					Second: 23,
				},
				Channels: []string{"system", "mqtt"},
			},
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
