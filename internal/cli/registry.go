package cli

import (
	"log/slog"

	"butler/internal/config"
	"butler/internal/notify"
)

func buildRegistry(config config.Config) *notify.Registry {
	registry := notify.NewRegistry()
	register := func(n notify.Notifier) {
		if err := registry.Register(n); err != nil {
			slog.Error("fail to register notifier", "error", err)
		} else {
			slog.Info("notifier registered", "channel", n.Name())
		}
	}

	register(notify.SystemNotifier{})

	register(notify.MessageboxNotifier{})

	mqttConfig := config.Mqtt
	if mqttConfig.Address != "" {
		mqttNotifier, err := notify.NewMqttNotifier(notify.MqttSetting{
			Address:    mqttConfig.Address,
			Topic:      mqttConfig.Topic,
			Username:   mqttConfig.Username,
			Password:   mqttConfig.Password,
			CertFile:   mqttConfig.CertFile,
			KeyFile:    mqttConfig.KeyFile,
			SkipVerify: mqttConfig.SkipVerify,
		})
		if err != nil {
			slog.Error("fail to make notifier", "channel", notify.ChannelMQTT, "err", err)
		} else {
			register(mqttNotifier)
		}
	}

	emailConfig := config.Email
	if emailConfig.Host != "" {
		emailNotifier, err := notify.NewEmailNotifier(emailConfig.Host, emailConfig.Port, emailConfig.Username, emailConfig.Authcode, emailConfig.From, emailConfig.To)
		if err != nil {
			slog.Error("fail to make notifier", "channel", notify.ChannelEmail, "err", err)
		} else {
			register(emailNotifier)
		}
	}
	return registry
}
