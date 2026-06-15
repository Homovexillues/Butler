package notify

import mqtt "github.com/eclipse/paho.mqtt.golang"

type MqttNotifier struct {
	Client mqtt.Client
}

func (sys MqttNotifier) Name() string {
	return "MqttNotifier"
}

func (sys MqttNotifier) Send(message Message) error {
	return nil
}
