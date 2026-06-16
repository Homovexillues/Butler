// Package cli provides various parameter calling methods and descriptions.
package cli

import (
	"context"
	"log"
	"os"
	"time"

	"butler/internal/engine"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "butler",
	Short: "A cyber butler which can scheduled notify",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Fail to execute butler command:%s", err.Error())
		os.Exit(1)
	}
	nodes := []*model.Node{
		{
			Title: "通知测试",
			Schedule: schedule.Once{
				At: time.Now().Add(10 * time.Second),
			},
		},
	}
	ctx := context.Background()
	notifier := notify.SystemNotifier{}
	engine.Run(ctx, nodes, notifier)
}
