package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	"butler/internal/api"
	"butler/internal/config"
	"butler/internal/engine"
	"butler/internal/notify"
	"butler/internal/parser"

	"github.com/kardianos/service"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

type program struct {
	cancel   context.CancelFunc
	server   *http.Server
	closeLog func() error
}

var svc service.Service

func (p *program) Start(s service.Service) error {
	err := ensureHome()
	if err != nil {
		return fmt.Errorf("fail to ensure home directory:\n%w", err)
	}
	closeFn, err := setupLogger()
	if err != nil {
		return fmt.Errorf("fail to setup logger:\n%w", err)
	}
	p.closeLog = closeFn
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("fail to load config:\n%w", err)
	}

	plan, err := config.LoadPlan()
	if err != nil {
		return fmt.Errorf("fail to load plan:\n%w", err)
	}
	nodes, err := parser.PlanToNodes(plan)
	if err != nil {
		return fmt.Errorf("fail to convert plan to nodes:\n%w", err)
	}

	registry := buildRegistry(cfg)

	requests := make(chan notify.Request, 10)

	ctx, cancel := context.WithCancel(context.Background())
	go notify.MessageLoop(ctx, registry, requests, 4)
	p.cancel = cancel

	p.server = &http.Server{
		Addr:              ":8191",
		Handler:           api.NewRouter(nodes),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		err := p.server.ListenAndServe()
		switch {
		case errors.Is(err, http.ErrServerClosed):
			slog.Info("HTTP server stopped")
		case err != nil:
			slog.Error("HTTP server failed", "error", err)

		}
	}()

	go engine.Run(ctx, nodes, requests, func() error { return config.SavePlan(plan) })

	return nil
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.server.Shutdown(ctx)
	}
	if p.closeLog != nil {
		return p.closeLog()
	}
	return nil
}

// 在用systemd启动的场景下，
// 往往会因为不加载全部的环境变量而找不到HOME,
// 而配置文件就依靠这个环境变量
func ensureHome() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if os.Getenv("HOME") != "" {
		return nil
	}
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		err := os.Setenv("HOME", u.HomeDir)
		if err != nil {
			return err
		}
	}
	return nil
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

func setupLogger() (closeFn func() error, err error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(configDir, "butler", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	rl, err := rotatelogs.New(
		filepath.Join(logDir, "%Y-%m-%d.log"),
		rotatelogs.WithRotationTime(24*time.Hour),
		rotatelogs.WithMaxAge(7*24*time.Hour),
		rotatelogs.WithClock(rotatelogs.Local))
	if err != nil {
		return nil, err
	}
	writer := io.MultiWriter(os.Stderr, rl)
	handler := slog.NewTextHandler(
		writer,
		&slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true,
		},
	)
	slog.SetDefault(slog.New(handler))
	return rl.Close, nil
}

func init() {
	var err error
	svc, err = newService("")
	if err != nil {
		log.Fatal(err)
	}
	rootCmd.AddCommand(installCmd, uninstallCmd, startCmd, stopCmd)
}
