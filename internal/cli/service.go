package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
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
	cancel       context.CancelFunc
	server       *http.Server
	closeDB      func() error
	closeChannel func() error
	closeLog     func() error
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

	store, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("fail to load database:\n%w", err)
	}
	p.closeDB = store.Close
	ctx, cancel := context.WithCancel(context.Background())
	configLoadedAt := time.Now().UTC()
	cfg, err := config.LoadConfig(ctx, store)
	if err != nil {
		cancel()
		return fmt.Errorf("fail to load config:\n%w", err)
	}

	registry := buildRegistry(cfg)
	p.closeChannel = registry.Close
	requests := make(chan notify.Request, 10)

	initNodes, err := loadNodes(ctx, store)
	if err != nil {
		cancel()
		return fmt.Errorf("fail to load nodes from database:\n%w", err)
	}

	nodesChannel := make(chan []*model.Node, 1)
	nodesChannel <- initNodes

	p.cancel = cancel

	if err = LoadHTTPServer(p, store, configLoadedAt); err != nil {
		return err
	}

	LoadMessageLoop(ctx, registry, requests, 4)

	LoadStoreCallback(ctx, nodesChannel, store)

	go engine.Run(ctx, nodesChannel, requests, func(id int64, lastAttempt *time.Time, lastFired *time.Time) error {
		return store.SaveRunState(ctx, id, lastAttempt, lastFired)
	})
	return nil
}

func PrintMem() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms) // 读取内存快照，会短暂STW

	fmt.Printf("==== Go GC堆信息（等价C#托管堆）====\n")
	fmt.Printf("HeapAlloc: %.2f MB 【正在使用的GC堆对象，最常用】\n", float64(ms.HeapAlloc)/1024/1024)
	fmt.Printf("HeapSys:   %.2f MB 【向OS申请的GC堆总内存（含空闲）】\n", float64(ms.HeapSys)/1024/1024)
	fmt.Printf("HeapIdle:  %.2f MB 【GC堆中空闲，还没还给OS的内存】\n", float64(ms.HeapIdle)/1024/1024)
	fmt.Printf("HeapReleased: %.2f MB 【已经归还操作系统的内存】\n", float64(ms.HeapReleased)/1024/1024)
	fmt.Printf("NumGC: %d 【已经执行GC次数】\n\n", ms.NumGC)
}

func LoadHTTPServer(p *program, s *store.Store, configLoadedAt time.Time) (err error) {
	ln, err := net.Listen("tcp", ":8191")
	if err != nil {
		err = fmt.Errorf("listen on 8191 : %w", err)
	}
	p.server = &http.Server{
		Addr:              ":8191",
		Handler:           api.NewRouter(s, configLoadedAt),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		err := p.server.Serve(ln)
		switch {
		case errors.Is(err, http.ErrServerClosed):
			slog.Info("HTTP server stopped")
		case err != nil:
			slog.Error("HTTP server failed", "error", err)

		}
	}()
	return err
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
			case t, ok := <-changes:
				if !ok {
					return
				}
				if t != reflect.TypeFor[model.Task]() {
					continue
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
	if p.closeDB != nil {
		if err := p.closeDB(); err != nil {
			return err
		}
	}
	if p.closeChannel != nil {
		if err := p.closeChannel(); err != nil {
			return err
		}
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
	tasks, err := store.GetAll[model.Task](ctx, s)
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

func getExecutablePath() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "/usr/local/bin/butler", nil
	case "windows":
		dir := os.Getenv("ProgramFiles")
		if dir == "" || !filepath.IsAbs(dir) {
			return "", fmt.Errorf("invalid ProgramFiles directory")
		}
		return filepath.Join(dir, "Butler", "butler.exe"), nil
	default:
		return "", fmt.Errorf("unsupported platform %q", runtime.GOOS)
	}
}

func copyExecutable(dst string) (err error) {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}
	inFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := inFile.Close()
		if err == nil {
			err = closeErr
		}
	}()
	srcInfo, err := inFile.Stat()
	if err != nil {
		return err
	}
	dstInfo, statErr := os.Stat(dst)
	if statErr == nil {
		if os.SameFile(srcInfo, dstInfo) {
			return nil
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o0755); err != nil {
		return err
	}
	outFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := outFile.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(dst)
		}
	}()

	if _, err = io.Copy(outFile, inFile); err != nil {
		return err
	}
	return outFile.Sync()
}

func newService(userName, executable string) (service.Service, error) {
	cfg := &service.Config{
		Name:        "butler",
		DisplayName: "Butler 电子管家 顾霈圭",
		Description: "定时调度通知服务",
		Arguments:   []string{"serve"},
		Executable:  executable,
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
	path, err := getExecutablePath()
	if err != nil {
		log.Fatal(err)
	}
	svc, err = newService("", path)
	if err != nil {
		log.Fatal(err)
	}
	rootCmd.AddCommand(installCmd, uninstallCmd, startCmd, stopCmd, updateCmd)
}
