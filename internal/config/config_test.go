package config

import (
	"butler/internal/model"
	"butler/internal/notify"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadConfigJSONCAndCertificatePath(t *testing.T) {
	dir := isolatedConfigRoot(t)
	raw := `{// comment
 "mqtt":{"address":":8883","topic":"test","certfile":"certs/server.crt","keyfile":"certs/server.key"},
 "email":{"host":"smtp.example.test"},
 }`
	if err := os.WriteFile(filepath.Join(dir, "config.jsonc"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mqtt.Address != ":8883" || cfg.Mqtt.Topic != "test" || cfg.Mqtt.CertFile != filepath.Join(dir, "certs/server.crt") || cfg.Email.Host != "smtp.example.test" {
		t.Fatalf("config=%+v", cfg)
	}
	// Existing user configuration must not be overwritten by loading it.
	got, err := os.ReadFile(filepath.Join(dir, "config.jsonc"))
	if err != nil || string(got) != raw {
		t.Fatalf("configuration overwritten: %v", err)
	}
}

func TestLoadConfigRejectsEmptyOrMalformed(t *testing.T) {
	dir := isolatedConfigRoot(t)
	for _, raw := range []string{"", "{bad"} {
		if err := os.WriteFile(filepath.Join(dir, "config.jsonc"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadConfig(); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	if errs := (Config{}).ValidateConfig(); len(errs) != 0 {
		t.Fatal(errs)
	}
	valid := Config{Mqtt: MqttSettings{Address: ":8883", Topic: "test"}, Email: EmailSettings{Host: "smtp.example.test", Port: 465, Username: "user", Authcode: "secret", From: "from@example.test", To: []string{"to@example.test"}}}
	if errs := valid.ValidateConfig(); len(errs) != 0 {
		t.Fatal(errs)
	}
	errs := (Config{Mqtt: MqttSettings{Username: "user"}, Email: EmailSettings{Host: "smtp.example.test"}}).ValidateConfig()
	for _, prefix := range []string{"mqtt:", "email:"} {
		found := false
		for _, err := range errs {
			if strings.Contains(err.Error(), prefix) {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s errors: %v", prefix, errs)
		}
	}
}

func TestLoadDatabasePersistsAcrossReopen(t *testing.T) {
	isolatedConfigRoot(t)
	s, err := LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Add(context.Background(), model.Task{Title: "persisted", Channels: []notify.Channel{notify.ChannelSystem}, ActionExpression: json.RawMessage(`{}`)})
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
	task, err := s.Get(context.Background(), id)
	if err != nil || task.Title != "persisted" {
		t.Fatalf("got %+v, %v", task, err)
	}
}
