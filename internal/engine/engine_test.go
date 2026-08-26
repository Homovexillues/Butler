package engine

import (
	"context"
	"testing"
	"time"

	"butler/internal/action"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
)

// TestRun_触发最近的Once并产生通知请求 验证 Engine 会等待异步 Action
// 完成、接收 actionResult，并在成功后更新 LastFired。
func TestRun_触发最近的Once并产生通知请求(t *testing.T) {
	triggerAt := time.Now().Add(50 * time.Millisecond)
	lastFired := time.Time{}
	node := &model.Node{
		Title:     "测试通知",
		Schedule:  schedule.Once{At: triggerAt},
		LastFired: &lastFired,
		Action: action.NotifyAction{
			Channels: []string{"system"},
			Message: notify.Message{
				Title: "测试通知",
				Body:  "测试正文",
			},
		},
	}

	requests := make(chan notify.Request, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	saved := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		Run(ctx, []*model.Node{node}, requests, func() error {
			saved <- struct{}{}
			return nil
		})
		close(done)
	}()

	select {
	case request := <-requests:
		if len(request.Channels) != 1 || request.Channels[0] != "system" {
			t.Errorf("通知渠道 = %v，期望 [system]", request.Channels)
		}
		if request.Message.Title != "测试通知" {
			t.Errorf("通知标题 = %q，期望 测试通知", request.Message.Title)
		}
	case <-ctx.Done():
		t.Fatal("Action 没有在超时前产生通知请求")
	}

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("Run 未在超时内结束，可能没有处理 Action 的执行结果")
	}

	if node.LastFired == nil || node.LastFired.IsZero() {
		t.Fatal("Action 成功后没有更新 LastFired")
	}
	if node.LastFired.Before(triggerAt) {
		t.Fatalf("LastFired = %s，早于计划触发时间 %s", node.LastFired, triggerAt)
	}

	select {
	case <-saved:
	default:
		t.Fatal("Action 成功后没有调用状态保存函数")
	}
}

// TestRun_ctx取消能干净退出 验证等待未来任务时，ctx 取消可以立即停止 Engine。
func TestRun_ctx取消能干净退出(t *testing.T) {
	lastFired := time.Time{}
	node := &model.Node{
		Title:     "未来任务",
		Schedule:  schedule.Once{At: time.Now().Add(time.Hour)},
		LastFired: &lastFired,
		Action: action.NotifyAction{
			Channels: []string{"system"},
			Message:  notify.Message{Title: "未来任务"},
		},
	}

	requests := make(chan notify.Request, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		Run(ctx, []*model.Node{node}, requests, func() error { return nil })
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ctx 取消后 Run 未及时退出")
	}

	select {
	case request := <-requests:
		t.Errorf("未来任务不应产生通知请求，实际得到 %+v", request)
	default:
	}
}
