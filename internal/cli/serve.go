package cli

import (
	"context"
	"log"

	"butler/internal/config"
	"butler/internal/engine"
	"butler/internal/notify"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "前台运行调度器",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.LoadConfig()
		if err != nil {
			log.Fatalf("Fail to load config:%s", err.Error())
		}
		nodes, err := config.LoadPlan()
		if err != nil {
			log.Fatalf("Fail to parse plan:%s", err.Error())
		}
		ctx := context.Background()

		registry := notify.NewRegistry()

		registry.Register(notify.SystemNotifier{})

		opts := mqtt.NewClientOptions()
		opts.AddBroker("tcp://127.0.0.1:1883")
		client := mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			log.Fatal(token.Error())
		}
		registry.Register(notify.MqttNotifier{
			Client: client,
			Topic:  "Notify",
		})

		engine.Run(ctx, registry, nodes)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
