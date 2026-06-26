package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"runtime"

	"butler/internal/config"
	"butler/internal/engine"
	"butler/internal/parser"

	"github.com/kardianos/service"
)

type program struct {
	cancel context.CancelFunc
}

var svc service.Service

func (p *program) Start(s service.Service) error {
	ensureHome()
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("fail to load config:\n%w", err)
	}

	plan, err := config.LoadPlan()
	if err != nil {
		return fmt.Errorf("fail to load plan:\n%w", err)
	}
	nodes, err := parser.PlanToNodes(*plan)
	if err != nil {
		return fmt.Errorf("fail to convert plan to nodes:\n%w", err)
	}

	registry := buildRegistry(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go engine.Run(ctx, registry, nodes)

	return nil
}

func (p *program) Stop(s service.Service) error {
	p.cancel()
	return nil
}

// 在用systemd启动的场景下，
// 往往会因为不加载全部的环境变量而找不到HOME,
// 而配置文件就依靠这个环境变量
func ensureHome() {
	if runtime.GOOS == "windows" {
		return
	}
	if os.Getenv("HOME") != "" {
		return
	}
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		err := os.Setenv("HOME", u.HomeDir)
		if err != nil {
			log.Printf("%s", err.Error())
		}
	}
}

func newService(userName string) (service.Service, error) {
	cfg := &service.Config{
		Name:        "butler",
		DisplayName: "Butler 电子管家 顾霈圭",
		Description: "定时调度通知服务",
		Arguments:   []string{"serve"},
		UserName:    userName, // 空字符串时 kardianos 不写 User=
	}
	return service.New(&program{}, cfg)
}

func init() {
	var err error
	svc, err = newService("")
	if err != nil {
		log.Fatal(err)
	}
	rootCmd.AddCommand(installCmd, uninstallCmd, startCmd, stopCmd)
}
