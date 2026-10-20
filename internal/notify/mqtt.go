package notify

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

type mqttNotifier struct {
	Server *mqtt.Server
	Topic  string
}

type MqttSetting struct {
	UpdatedAt  time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
	ID         int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Address    string    `json:"address" gorm:"column:address;not null"`
	Topic      string    `json:"topic" gorm:"column:topic;not null"`
	ClientID   string    `json:"clientId" gorm:"column:client_id"`
	Username   string    `json:"username" gorm:"column:username;not null"`
	Password   string    `json:"password" gorm:"column:password;not null"`
	CertFile   string    `json:"certfile" gorm:"column:cert_file"`
	KeyFile    string    `json:"keyfile" gorm:"column:key_file"`
	SkipVerify bool      `json:"skipverify" gorm:"column:skip_verify;not null"`
}

const qosAtleastOnce = 1

// Validate checks settings without binding a port or starting a broker.
func (settings MqttSetting) Validate() error {
	if settings.Address == "" || settings.Topic == "" || settings.Username == "" || settings.Password == "" {
		return fmt.Errorf("mqtt configuration is not complete")
	}
	_, port, err := net.SplitHostPort(settings.Address)
	if err != nil {
		return fmt.Errorf("mqtt: invalid listen address: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("mqtt: port must be between 1 and 65535")
	}
	if strings.ContainsAny(settings.Topic, "+#\x00") {
		return fmt.Errorf("mqtt: topic must not contain wildcards or NUL")
	}
	if !settings.SkipVerify {
		if _, err := tls.LoadX509KeyPair(settings.CertFile, settings.KeyFile); err != nil {
			return fmt.Errorf("mqtt: load certificate: %w", err)
		}
	}
	return nil
}

func NewMqttNotifier(settings MqttSetting) (Notifier, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	var tlsConfig *tls.Config
	if !settings.SkipVerify {
		cert, err := tls.LoadX509KeyPair(settings.CertFile, settings.KeyFile)
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
				settings.Username: {
					Password: auth.RString(settings.Password),
					ACL: auth.Filters{
						auth.RString(settings.Topic): auth.ReadOnly,
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
		Address:   settings.Address,
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
	return &mqttNotifier{Server: broker, Topic: settings.Topic}, nil
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
