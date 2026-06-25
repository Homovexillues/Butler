// Package config is used to configuation the notify plan and user config
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"butler/internal/parser"
)

type Config struct {
	Email EmailSettings `json:"email"`
	Mqtt  MqttSettings  `json:"mqtt"`
}

type MqttSettings struct {
	Broker     string `json:"broker"`
	Topic      string `json:"topic"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	CertFile   string `json:"certfile"`
	SkipVerify bool   `json:"skipverify"`
}
type EmailSettings struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Authcode string   `json:"authcode"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

const (
	EveryoneReadAndOwnerWrite      = 0o644
	OwnerReadAndWrite              = 0o600
	OwnerAllAndEveryoneReadExecute = 0o755
)

func LoadConfig() (Config, error) {
	configDir, err := ensureDirectory()
	if err != nil {
		return Config{}, err
	}
	configPath := filepath.Join(configDir, "config.jsonc")
	err = ensureFile(configPath, OwnerReadAndWrite)
	if err != nil {
		return Config{}, err
	}
	config, err := parser.Parse[Config](configPath)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}

func ensureDirectory() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	configDir = filepath.Join(configDir, "butler")
	err = os.MkdirAll(configDir, OwnerAllAndEveryoneReadExecute)
	if err != nil {
		return "", err
	}
	return configDir, nil
}

func LoadPlan() (*parser.Plan, error) {
	configDir, err := ensureDirectory()
	if err != nil {
		return nil, err
	}
	planPath := filepath.Join(configDir, "plan.jsonc")
	err = ensureFile(planPath, EveryoneReadAndOwnerWrite)
	if err != nil {
		return nil, err
	}
	plan, err := parser.Parse[parser.Plan](planPath)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func ensureFile(path string, fileMode os.FileMode) error {
	// os.O_CREATE: 如果文件不存在则创建
	// os.O_EXCL: 与 O_CREATE 一起使用，文件必须不存在，否则返回错误
	// os.O_RDONLY: 以只读方式打开
	_, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		// 是文件存在类型的错误
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func (cfg Config) ValidateConfig() []error {
	var errs []error
	errs = append(errs, cfg.Mqtt.validate()...)
	errs = append(errs, cfg.Email.validate()...)
	return errs
}

// validate 检查 mqtt 配置：仅当该渠道被配置（任一字段非空）时才校验必填项，
// 未配置则视为不使用该渠道，跳过。username/password 可选（允许匿名 broker），
// certfile 可选（SkipVerify 时不需要）。
func (m MqttSettings) validate() []error {
	configured := m.Broker != "" || m.Topic != "" || m.Username != "" ||
		m.Password != "" || m.CertFile != "" || m.SkipVerify
	if !configured {
		return nil
	}
	var errs []error
	required := []struct {
		name string
		ok   bool
	}{
		{"broker", m.Broker != ""},
		{"topic", m.Topic != ""},
	}
	for _, r := range required {
		if !r.ok {
			errs = append(errs, fmt.Errorf("mqtt: %s 不能为空", r.name))
		}
	}
	return errs
}

// validate 检查 email 配置：同样仅在被配置时校验。email 各字段都是发信必需的。
func (e EmailSettings) validate() []error {
	configured := e.Host != "" || e.Username != "" || e.Authcode != "" ||
		e.From != "" || len(e.To) > 0 || e.Port != 0
	if !configured {
		return nil
	}
	var errs []error
	required := []struct {
		name string
		ok   bool
	}{
		{"host", e.Host != ""},
		{"port", e.Port > 0},
		{"username", e.Username != ""},
		{"authcode", e.Authcode != ""},
		{"from", e.From != ""},
		{"to", len(e.To) > 0},
	}
	for _, r := range required {
		if !r.ok {
			errs = append(errs, fmt.Errorf("email: %s 不能为空", r.name))
		}
	}
	return errs
}
