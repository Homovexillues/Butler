// Package config is used to configuation the notify plan and user config
package config

import (
	"os"
	"path/filepath"

	"butler/internal/model"
	"butler/internal/parser"
	"butler/internal/store"
)

const (
	EveryoneReadAndOwnerWrite      = 0o644
	OwnerReadAndWrite              = 0o600
	OwnerAllAndEveryoneReadExecute = 0o755
)

func LoadConfig() (model.Config, error) {
	configDir, err := ensureDirectory()
	if err != nil {
		return model.Config{}, err
	}
	configPath := filepath.Join(configDir, "config.jsonc")
	err = ensureFile(configPath, OwnerReadAndWrite)
	if err != nil {
		return model.Config{}, err
	}
	config, err := parser.Parse[model.Config](configPath)
	if err != nil {
		return model.Config{}, err
	}
	if config.Mqtt.CertFile != "" && !filepath.IsAbs(config.Mqtt.CertFile) {
		config.Mqtt.CertFile = filepath.Join(configDir, config.Mqtt.CertFile)
	}
	return config, nil
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
