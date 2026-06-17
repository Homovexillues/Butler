package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
)

// recordingNotifier 是测试用假渠道：记录被触发的次数与收到的消息标题。
// 并发安全，因为 Broadcast 会用 goroutine 调用 Send。
type recordingNotifier struct {
	name string

	mu     sync.Mutex
	titles []string
}

func (r *recordingNotifier) Name() string { return r.name }

func (r *recordingNotifier) Send(ctx context.Context, msg notify.Message) error {
	r.mu.Lock()
	r.titles = append(r.titles, msg.Title)
	r.mu.Unlock()
	return nil
}

func (r *recordingNotifier) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.titles)
}

// mockNodes 手搓一组节点，覆盖三种 schedule 类型，都挂到 "system" 渠道。
// 注意：Once 设成很近的未来，方便测试快速触发。
func mockNodes() []*model.Node {
	cronEveryMinute, _ := schedule.NewCronSchedule("* * * * *") // 每分钟
	return []*model.Node{
		{
			Title:    "马上提醒(Once)",
			Schedule: schedule.Once{At: time.Now().Add(50 * time.Millisecond)},
			Channels: []string{"system"},
		},
		{
			Title:    "娘的生日(SolarAnnual)",
			Schedule: schedule.SolarAnnual{Month: 3, Day: 5, Hour: 9},
			Channels: []string{"system"},
		},
		{
			Title:    "每分钟心跳(Cron)",
			Schedule: cronEveryMinute,
			Channels: []string{"system"},
		},
	}
}

// TestRun_触发最近的Once并广播 验证 engine 从一组节点里挑出最近的、到点广播。
func TestRun_触发最近的Once并广播(t *testing.T) {
	rec := &recordingNotifier{name: "system"}
	reg := notify.NewRegistry()
	reg.Register(rec)

	// 只放一个很快触发的 Once，触发后 NextAfter 返回 false，Run 自然退出
	nodes := []*model.Node{
		{
			Title:    "测试通知",
			Schedule: schedule.Once{At: time.Now().Add(50 * time.Millisecond)},
			Channels: []string{"system"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		Run(ctx, reg, nodes)
		close(done)
	}()

	select {
	case <-done:
		// Run 在 Once 触发后应自行返回（再无未来节点）
	case <-ctx.Done():
		t.Fatal("Run 未在超时内结束，可能没触发或没退出")
	}

	if rec.count() != 1 {
		t.Fatalf("期望广播 1 次，实际 %d 次", rec.count())
	}
	if got := rec.titles[0]; got != "测试通知" {
		t.Errorf("广播标题 = %q，期望 测试通知", got)
	}
}

// TestRun_ctx取消能干净退出 验证没有可触发节点时阻塞，ctx 取消后立刻返回。
func TestRun_ctx取消能干净退出(t *testing.T) {
	rec := &recordingNotifier{name: "system"}
	reg := notify.NewRegistry()
	reg.Register(rec)

	// 节点都在遥远未来，Run 会进入长睡（被 maxTick 截断），靠 ctx 取消退出
	nodes := []*model.Node{
		{
			Title:    "明年的事",
			Schedule: schedule.SolarAnnual{Month: 1, Day: 1, Hour: 9},
			Channels: []string{"system"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		Run(ctx, reg, nodes)
		close(done)
	}()

	cancel() // 立即取消

	select {
	case <-done:
		// 期望：迅速退出
	case <-time.After(time.Second):
		t.Fatal("ctx 取消后 Run 未及时退出")
	}

	if rec.count() != 0 {
		t.Errorf("不该有任何广播，实际 %d 次", rec.count())
	}
}
