package notify

import (
	"context"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MqttNotifier struct {
	Client mqtt.Client
	Topic  string
}

func (mqttNotifier MqttNotifier) Name() string {
	return "mqtt"
}

func (mqttNotifier MqttNotifier) Send(ctx context.Context, message Message) error {
	token := mqttNotifier.Client.Publish(mqttNotifier.Topic, 2, true, message.Title)
	if token.Error() != nil {
		return token.Error()
	}
	return nil
}
