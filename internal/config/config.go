// Package config is used to configuation the notify plan and user config
package config

import (
	"os"
	"path/filepath"

	"butler/internal/model"
	"butler/internal/parser"
)

type Config struct {
	Email EmailSettings
	Mqtt  MqttSettings
}

type MqttSettings struct {
	Broker string
	Topic  string
}
type EmailSettings struct{}

const (
	EveryoneReadAndOwnerWrite      = 0o644
	OwnerReadAndWrite              = 0o600
	OwnerAllAndEveryoneReadExecute = 0o755
)

var (
	PlanPath   string
	ConfigPath string
)

func LoadConfig() (Config, error) {
	configDir, err := ensureDirectory()
	if err != nil {
		return Config{}, err
	}
	ConfigPath = filepath.Join(configDir, "config.jsonc")
	err = ensureFile(ConfigPath, OwnerReadAndWrite)
	if err != nil {
		return Config{}, err
	}
	config, err := parser.Parse[Config](ConfigPath)
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

func LoadPlan() ([]*model.Node, error) {
	configDir, err := ensureDirectory()
	if err != nil {
		return []*model.Node{}, err
	}
	PlanPath = filepath.Join(configDir, "plan.jsonc")
	err = ensureFile(PlanPath, EveryoneReadAndOwnerWrite)
	if err != nil {
		return []*model.Node{}, err
	}
	nodes, err := parser.Parse[[]*model.Node](PlanPath)
	if err != nil {
		return []*model.Node{}, err
	}
	return nodes, nil
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
