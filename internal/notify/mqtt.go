package notify

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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

func NewMqttNotifier(broker string, topic string, username string, password string, clientId string, crtFilePath string, insecure bool) (Notifier, error) {
	opts := mqtt.NewClientOptions()
	opts.SetConnectRetry(true)
	tlsConfig := &tls.Config{}
	protocol := "tcp"
	if username != "" {
		opts.SetUsername(username)
	}
	if password != "" {
		opts.SetPassword(password)
	}
	useTLS := false
	if insecure {
		useTLS = true
		tlsConfig.InsecureSkipVerify = insecure
	}
	if crtFilePath != "" {
		useTLS = true
		caCert, err := os.ReadFile(crtFilePath)
		if err != nil {
			return nil, fmt.Errorf("fail to read CA cert: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("fail to parse CA cert:%s", crtFilePath)
		}

		tlsConfig.RootCAs = caCertPool
	}
	if useTLS {
		protocol = "tls"
		opts.SetTLSConfig(tlsConfig)
	}
	address := protocol + "://" + broker
	opts.AddBroker(address)
	if clientId == "" {
		clientId = "butler"
	}
	opts.SetClientID(clientId)

	opts.SetConnectionNotificationHandler(func(_ mqtt.Client, event mqtt.ConnectionNotification) {
		switch e := event.(type) {
		case mqtt.ConnectionNotificationConnecting:
			slog.Debug("mqtt connecting", "attempt", e.Attempt, "reconnect", e.IsReconnect)
		case mqtt.ConnectionNotificationConnected:
			slog.Info("mqtt broker connected")
		case mqtt.ConnectionNotificationFailed:
			slog.Warn("mqtt connection failed", "error", e.Reason)
		case mqtt.ConnectionNotificationLost:
			slog.Warn("mqtt connection lost", "error", e.Reason)
		}
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(connectionTimeout) {
		return nil, fmt.Errorf("connection timeout %v", token.Error())
	}
	if token.Error() != nil {
		return nil, fmt.Errorf("fail to connect broker: %v", token.Error())
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
