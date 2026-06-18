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
	"butler/internal/notify"
	"butler/internal/schedule"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "前台运行调度器",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("fail to load config:\n%w", err)
		}
		cronSchedule, err := schedule.NewCronSchedule(" */1 * * * *")
		if err != nil {
			return fmt.Errorf("fail to make new CronSchedule:\n%w", err)
		}
		//nodes, err := config.LoadPlan()
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
		// if err != nil {
		// 	log.Fatalf("Fail to parse plan:%s", err.Error())
		// }
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		registry := notify.NewRegistry()

		registry.Register(notify.SystemNotifier{})

		mqttConfig := config.Mqtt
		mqttNotifier, err := notify.NewMqttNotifier(mqttConfig.Broker, mqttConfig.Topic)
		if err != nil {
			return fmt.Errorf("fail to make new MqttNotifier:\n%w", err)
		} else {
			registry.Register(mqttNotifier)
		}

		emailConfig := config.Email
		emailNotifier, err := notify.NewEmailNotifier(emailConfig.Host, emailConfig.Port, emailConfig.Username, emailConfig.Authcode, emailConfig.From, emailConfig.To)
		if err != nil {
			return fmt.Errorf("failt to make EmailNotifier:\n%w", err)
		} else {
			registry.Register(emailNotifier)
		}
		engine.Run(ctx, registry, nodes)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
