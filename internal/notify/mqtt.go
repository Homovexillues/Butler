package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type mqttNotifier struct {
	Client mqtt.Client
	Topic  string
}

const (
	qosAtleastOnce    = 1
	connectionTimeout = 3 * time.Second
)

func NewMqttNotifier(broker string, topic string) (Notifier, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(broker)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(connectionTimeout) {
		return mqttNotifier{}, fmt.Errorf("mqtt connection timeout:\n %w", token.Error())
	}
	if token.Error() != nil {
		return mqttNotifier{}, fmt.Errorf("fail to connection mqtt broker:\n %w", token.Error())
	}
	mqttNotifier := mqttNotifier{
		Client: client,
		Topic:  topic,
	}
	return mqttNotifier, nil
}

func (mqttNotifier mqttNotifier) Name() string {
	return "mqtt"
}

func (mqttNotifier mqttNotifier) Send(ctx context.Context, message Message) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	token := mqttNotifier.Client.Publish(mqttNotifier.Topic, qosAtleastOnce, false, payload)
	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		return ctx.Err()
	}
}
