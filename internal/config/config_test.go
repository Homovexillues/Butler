package config

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/store"
)

func isolatedConfigRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("APPDATA", root)
	t.Setenv("HOME", root)
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.Join(dir, "butler")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadConfigEmptyAndPersisted(t *testing.T) {
	isolatedConfigRoot(t)
	s, err := LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	cfg, err := LoadConfig(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mqtt.Address != "" || cfg.Email.Host != "" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	mqtt := notify.MqttSetting{ID: DefaultSettingID, Address: "127.0.0.1:1883", Topic: "test", Username: "user", Password: "secret", SkipVerify: true}
	email := notify.EmailNotifier{ID: DefaultSettingID, Host: "smtp.example.test", Port: 465, Username: "user", Authcode: "secret", From: "from@example.test", To: []string{"to@example.test"}}
	if err := store.SaveSetting(ctx, s, &mqtt); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSetting(ctx, s, &email); err != nil {
		t.Fatal(err)
	}
	// SQLite retains the instant and offset, not Go's location/monotonic metadata.
	mqtt.UpdatedAt = mqtt.UpdatedAt.UTC()
	email.UpdatedAt = email.UpdatedAt.UTC()
	cfg, err = LoadConfig(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Mqtt.UpdatedAt = cfg.Mqtt.UpdatedAt.UTC()
	cfg.Email.UpdatedAt = cfg.Email.UpdatedAt.UTC()
	if !reflect.DeepEqual(cfg.Mqtt, mqtt) || !reflect.DeepEqual(cfg.Email, email) {
		t.Fatalf("settings changed: %+v", cfg)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cfg, err = LoadConfig(ctx, s)
	cfg.Mqtt.UpdatedAt = cfg.Mqtt.UpdatedAt.UTC()
	cfg.Email.UpdatedAt = cfg.Email.UpdatedAt.UTC()
	if err != nil || !reflect.DeepEqual(cfg.Mqtt, mqtt) || !reflect.DeepEqual(cfg.Email, email) {
		t.Fatalf("settings not persisted: %+v, %v", cfg, err)
	}
}

func TestValidateConfigDoesNotStartBroker(t *testing.T) {
	// Occupy the address: validation must not attempt to listen on it.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg := Config{Mqtt: notify.MqttSetting{Address: listener.Addr().String(), Topic: "test", Username: "user", Password: "secret", SkipVerify: true}}
	if err := errors.Join(cfg.ValidateConfig()...); err != nil {
		t.Fatal(err)
	}
	cfg.Mqtt.Password = ""
	if err := errors.Join(cfg.ValidateConfig()...); err == nil {
		t.Fatal("invalid MQTT configuration accepted")
	}
}

func TestLoadDatabasePersistsAcrossReopen(t *testing.T) {
	isolatedConfigRoot(t)
	s, err := LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	want := model.Task{Title: "persisted", Channels: []notify.Channel{notify.ChannelSystem}, ActionExpression: json.RawMessage(`{}`)}
	err = store.Add(context.Background(), s, &want)
	if err != nil {
		if closeErr := s.Close(); closeErr != nil {
			t.Errorf("close database after failed insert: %v", closeErr)
		}
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	task, err := store.Get[model.Task](context.Background(), s, want.ID)
	if err != nil || task.Title != "persisted" {
		t.Fatalf("got %+v, %v", task, err)
	}
}
