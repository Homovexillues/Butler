package notify

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

type mqttNotifier struct {
	Server *mqtt.Server
	Topic  string
}

const qosAtleastOnce = 1

func NewMqttNotifier(listen, topic, username, password, certFile, keyFile string, insecure bool) (Notifier, error) {
	if listen == "" || topic == "" || username == "" || password == "" {
		return nil, fmt.Errorf("mqtt configuration is not complete")
	}
	var tlsConfig *tls.Config
	if !insecure {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("fail to load certificate file:%w", err)
		}
		tlsConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
	}
	broker := mqtt.New(&mqtt.Options{
		InlineClient: true,
	})
	err := broker.AddHook(new(auth.Hook), &auth.Options{
		Ledger: &auth.Ledger{
			Users: auth.Users{
				username: {
					Password: auth.RString(password),
					ACL: auth.Filters{
						auth.RString(topic): auth.ReadOnly,
					},
				},
			},
			ACL: auth.ACLRules{
				{Filters: auth.Filters{"#": auth.Deny}},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	err = broker.AddListener(listeners.NewTCP(listeners.Config{
		ID:        "mqtt-tls",
		Address:   listen,
		TLSConfig: tlsConfig,
	}))
	if err != nil {
		_ = broker.Close()
		return nil, err
	}
	if err = broker.Serve(); err != nil {
		_ = broker.Close()
		return nil, err
	}
	return &mqttNotifier{Server: broker, Topic: topic}, nil
}

func (mqttNotifier mqttNotifier) Close() error {
	return mqttNotifier.Server.Close()
}

func (mqttNotifier mqttNotifier) Name() string {
	return "mqtt"
}

func (mqttNotifier mqttNotifier) Send(ctx context.Context, message Message) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return mqttNotifier.Server.Publish(mqttNotifier.Topic, payload, false, qosAtleastOnce)
}
