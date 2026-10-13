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
	"sync"
	"time"

	"butler/internal/api"
	"butler/internal/config"
	"butler/internal/engine"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/parser"
	"butler/internal/store"

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

	store, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("fail to load database:\n%w", err)
	}

	registry := buildRegistry(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	requests := make(chan notify.Request, 10)

	initNodes, err := loadNodes(ctx, store)
	if err != nil {
		cancel()
		return fmt.Errorf("fail to load nodes from database:\n%w", err)
	}

	nodesChannel := make(chan []*model.Node, 1)
	nodesChannel <- initNodes

	p.cancel = cancel

	LoadMessageLoop(ctx, registry, requests, 4)

	LoadHTTPServer(p, store)

	LoadStoreCallback(ctx, nodesChannel, store)

	go engine.Run(ctx, nodesChannel, requests, func(id int64, lastFired time.Time) error {
		return store.MarkLastFired(ctx, id, lastFired)
	})

	return nil
}

func LoadHTTPServer(p *program, s *store.Store) {
	p.server = &http.Server{
		Addr:              ":8191",
		Handler:           api.NewRouter(s),
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
}

func LoadMessageLoop(ctx context.Context, registry *notify.Registry, requests <-chan notify.Request, workers int) {
	go func() {
		var wg sync.WaitGroup
		for range workers {
			wg.Go(
				func() {
					for {
						select {
						case request := <-requests:
							err := notify.Broadcast(ctx, registry, request.Channels, request.Message)
							if err != nil {
								slog.Error("fail to send message", "Title", request.Message.Title, "error", err)
							} else {
								slog.Info("succeed to send message", "Title", request.Message.Title)
							}
						case <-ctx.Done():
							return
						}
					}
				},
			)
		}
		wg.Wait()
	}()
}

func LoadStoreCallback(ctx context.Context, nodesChannel chan<- []*model.Node, s *store.Store) {
	go func() {
		defer close(nodesChannel)
		changes := s.Changes()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-changes:
				if !ok {
					return
				}
			}
			nodes, err := loadNodes(ctx, s)
			if err != nil {
				slog.Error("reload tasks failed", "error", err)
				continue
			}
			select {
			case nodesChannel <- nodes:
			case <-ctx.Done():
				return
			}
		}
	}()
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

func loadNodes(ctx context.Context, s *store.Store) ([]*model.Node, error) {
	tasks, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make([]*model.Node, 0, len(tasks))
	for _, task := range tasks {
		if !task.Enabled {
			continue
		}
		node, err := parser.TaskToNode(task)
		if err != nil {
			return nil, fmt.Errorf("task %d %w", task.ID, err)
		}
		nodes = append(nodes, &node)
	}
	return nodes, nil
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
	handler := slog.NewJSONHandler(
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
