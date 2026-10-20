// Package config is used to configuation the notify plan and user config
package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"butler/internal/notify"
	"butler/internal/store"

	"gorm.io/gorm"
)

type Config struct {
	Mqtt  notify.MqttSetting
	Email notify.EmailNotifier
}

const (
	EveryoneReadAndOwnerWrite      = 0o644
	OwnerReadAndWrite              = 0o600
	OwnerAllAndEveryoneReadExecute = 0o755
	DefaultSettingID               = 1
)

func (cfg *Config) ValidateConfig() []error {
	var errs []error
	if cfg.Mqtt.Address != "" {
		if err := cfg.Mqtt.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if cfg.Email.Host != "" {
		_, err := notify.NewEmailNotifier(cfg.Email.Host, cfg.Email.Port, cfg.Email.Username, cfg.Email.Authcode, cfg.Email.From, cfg.Email.To)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func LoadConfig(ctx context.Context, s *store.Store) (Config, error) {
	var cfg Config
	// 目前不考虑多套配置同时使用，故暂定使用默认配置
	mqttSetting, err := store.Get[notify.MqttSetting](ctx, s, DefaultSettingID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Config{}, err
		}
		mqttSetting = notify.MqttSetting{ID: DefaultSettingID}
	}
	cfg.Mqtt = mqttSetting
	email, err := store.Get[notify.EmailNotifier](ctx, s, DefaultSettingID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return Config{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		email = notify.EmailNotifier{ID: DefaultSettingID}
	}
	cfg.Email = email
	return cfg, nil
}

func LoadDatabase() (*store.Store, error) {
	configDir, err := ensureDirectory()
	if err != nil {
		return nil, err
	}
	databasePath := filepath.Join(configDir, "butler.db")
	butlerStore, err := store.NewStore(databasePath)
	if err != nil {
		return nil, err
	}
	return butlerStore, nil
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

func ensureFile(path string, fileMode os.FileMode) error {
	// os.O_CREATE: 如果文件不存在则创建
	// os.O_EXCL: 与 O_CREATE 一起使用，文件必须不存在，否则返回错误
	// os.O_RDONLY: 以只读方式打开
	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		// 是文件存在类型的错误
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return file.Close()
}
