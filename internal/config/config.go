// Package config is used to configuation the notify plan and user config
package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	PlanPath   string
	ConfigPath string
}

const (
	EveryoneReadAndOwnerWrite      = 0o644
	OwnerReadAndWrite              = 0o600
	OwnerAllAndEveryoneReadExecute = 0o755
)

func Load() (Config, error) {
	config := Config{}
	configDir, err := os.UserConfigDir()
	configDir = filepath.Join(configDir, "butler")
	if err != nil {
		return Config{}, err
	}
	err = os.MkdirAll(configDir, OwnerAllAndEveryoneReadExecute)
	if err != nil {
		return Config{}, err
	}
	config.PlanPath = filepath.Join(configDir, "plan.jsonc")
	config.ConfigPath = filepath.Join(configDir, "config.jsonc")

	err = ensureFile(config.PlanPath, EveryoneReadAndOwnerWrite)
	if err != nil {
		return Config{}, err
	}
	err = ensureFile(config.ConfigPath, OwnerReadAndWrite)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}

func ensureFile(path string, fileMode os.FileMode) error {
	// os.O_CREATE: 如果文件不存在则创建
	// os.O_EXCL: 与 O_CREATE 一起使用，文件必须不存在，否则返回错误
	// os.O_RDONLY: 以只读方式打开
	// 0666 读写权限，竟然这里用魔术数字，他妈的
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
