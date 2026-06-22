package cli

import (
	"context"
	"fmt"
	"time"

	"butler/internal/config"
	"butler/internal/notify"

	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test <channel>",
	Short: "立即向指定渠道发送一条测试消息",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		channel := args[0]
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("fail to load config:\n%w", err)
		}

		registry := buildRegistry(config)

		notifier, ok := registry.Get(channel)
		if !ok {
			return fmt.Errorf("fail to get channel %s from registry tab", channel)
		}
		ctx, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()

		message := notify.Message{
			Title: "butler 测试消息",
			Body:  fmt.Sprintf("这是一条来自 butler test 的测试 (%s)", channel),
		}
		if err := notifier.Send(ctx, message); err != nil {
			return fmt.Errorf("fail to send message by channel %s:\n%w", channel, err)
		}
		fmt.Printf("✓ 已通过 %s 发送测试消息\n", channel)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
