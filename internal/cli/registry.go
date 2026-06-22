package cli

import (
	"log"

	"butler/internal/config"
	"butler/internal/notify"
)

func buildRegistry(config config.Config) *notify.Registry {
	registry := notify.NewRegistry()

	registry.Register(notify.SystemNotifier{})

	mqttConfig := config.Mqtt
	mqttNotifier, err := notify.NewMqttNotifier(mqttConfig.Broker, mqttConfig.Topic)
	if err != nil {
		log.Printf("fail to make new MqttNotifier:\n%s", err.Error())
	} else {
		registry.Register(mqttNotifier)
	}

	emailConfig := config.Email
	emailNotifier, err := notify.NewEmailNotifier(emailConfig.Host, emailConfig.Port, emailConfig.Username, emailConfig.Authcode, emailConfig.From, emailConfig.To)
	if err != nil {
		log.Printf("fail to make EmailNotifier:\n%s", err.Error())
	} else {
		registry.Register(emailNotifier)
	}
	return registry
}
