package cli

import (
	"log/slog"

	"butler/internal/config"
	"butler/internal/notify"
)

func buildRegistry(config config.Config) *notify.Registry {
	registry := notify.NewRegistry()

	registry.Register(notify.SystemNotifier{})

	registry.Register(notify.MessageboxNotifier{})

	mqttConfig := config.Mqtt
	mqttNotifier, err := notify.NewMqttNotifier(mqttConfig.Broker, mqttConfig.Topic, mqttConfig.Username, mqttConfig.Password, mqttConfig.ClientID, mqttConfig.CertFile, mqttConfig.SkipVerify)
	if err != nil {
		slog.Error("fail to make notifier", "channel", notify.ChannelMQTT, "err", err)
	} else {
		slog.Info("notifier registered", "channel", notify.ChannelMQTT)
		registry.Register(mqttNotifier)
	}

	emailConfig := config.Email
	emailNotifier, err := notify.NewEmailNotifier(emailConfig.Host, emailConfig.Port, emailConfig.Username, emailConfig.Authcode, emailConfig.From, emailConfig.To)
	if err != nil {
		slog.Error("fail to make notifier", "channel", notify.ChannelEmail, "err", err)
	} else {
		slog.Info("notifier registered", "channel", notify.ChannelEmail)
		registry.Register(emailNotifier)
	}
	return registry
}
