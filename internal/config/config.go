// Package config is used to configuation the notify plan and user config
package config

import (
	"log"
	"os"
	"path/filepath"
)

var (
	PlanPath   string
	ConfigPath string
)

func init() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("Fail to detect user config dictionary:%s", err.Error())
	}
	PlanPath = filepath.Join(configDir, "plan.jsonc")
	ConfigPath = filepath.Join(configDir, "plan.jsonc")

	err = ensureFile(PlanPath)
	if err != nil {
		log.Fatalf("Fail to ensure plan file exist:%s", err.Error())
	}
	err = ensureFile(ConfigPath)
	if err != nil {
		log.Fatalf("Fail to ensure config file exist:%s", err.Error())
	}
}

func ensureFile(path string) error {
	// os.O_CREATE: 如果文件不存在则创建
	// os.O_EXCL: 与 O_CREATE 一起使用，文件必须不存在，否则返回错误
	// os.O_RDONLY: 以只读方式打开
	// 0666 读写权限，竟然这里用魔术数字，他妈的
	readAndWrite := 0o666
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE|os.O_EXCL, os.FileMode(readAndWrite))
	if err != nil {
		// 是文件存在类型的错误
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	return nil
}
