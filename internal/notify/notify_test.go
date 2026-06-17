package notify

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// mockNotifier 是测试用的假渠道：可配置名字、是否失败，并记录收到的消息。
// 用 mutex 保护是因为 Broadcast 会并发调用 Send（多 goroutine）。
type mockNotifier struct {
	name     string
	failWith error // 非 nil 则 Send 返回该错误

	mu       sync.Mutex
	calls    int
	gotMsgs  []Message
}

func (m *mockNotifier) Name() string { return m.name }

func (m *mockNotifier) Send(ctx context.Context, msg Message) error {
	m.mu.Lock()
	m.calls++
	m.gotMsgs = append(m.gotMsgs, msg)
	m.mu.Unlock()
	return m.failWith
}

func (m *mockNotifier) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// ---- Registry ----

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	sys := &mockNotifier{name: "system"}
	reg.Register(sys)

	t.Run("已注册_能取到", func(t *testing.T) {
		got, ok := reg.Get("system")
		if !ok {
			t.Fatal("期望 ok=true，得到 false")
		}
		if got.Name() != "system" {
			t.Errorf("取回的渠道名 = %q，期望 system", got.Name())
		}
	})

	t.Run("未注册_返回false", func(t *testing.T) {
		if _, ok := reg.Get("mqtt"); ok {
			t.Error("未注册的渠道应返回 ok=false")
		}
	})

	t.Run("同名注册_后者覆盖前者", func(t *testing.T) {
		reg := NewRegistry()
		reg.Register(&mockNotifier{name: "x", failWith: errors.New("旧")})
		newer := &mockNotifier{name: "x"}
		reg.Register(newer)

		got, _ := reg.Get("x")
		if got != newer {
			t.Error("同名 Register 应覆盖为最新实例")
		}
	})
}

// ---- Broadcast ----

func TestBroadcast(t *testing.T) {
	ctx := context.Background()
	msg := Message{Title: "标题", Body: "正文"}

	t.Run("多渠道_每个都收到一次", func(t *testing.T) {
		sys := &mockNotifier{name: "system"}
		mqtt := &mockNotifier{name: "mqtt"}
		reg := NewRegistry()
		reg.Register(sys)
		reg.Register(mqtt)

		Broadcast(ctx, reg, []string{"system", "mqtt"}, msg)

		if sys.callCount() != 1 {
			t.Errorf("system 被调用 %d 次，期望 1", sys.callCount())
		}
		if mqtt.callCount() != 1 {
			t.Errorf("mqtt 被调用 %d 次，期望 1", mqtt.callCount())
		}
		// 内容正确传递
		if len(sys.gotMsgs) == 1 && sys.gotMsgs[0] != msg {
			t.Errorf("system 收到 %+v，期望 %+v", sys.gotMsgs[0], msg)
		}
	})

	t.Run("单渠道失败_不影响其他渠道", func(t *testing.T) {
		bad := &mockNotifier{name: "mqtt", failWith: errors.New("broker 挂了")}
		good := &mockNotifier{name: "email"}
		reg := NewRegistry()
		reg.Register(bad)
		reg.Register(good)

		// bad 失败只应被 log，不影响 good，也不 panic / 退出
		Broadcast(ctx, reg, []string{"mqtt", "email"}, msg)

		if bad.callCount() != 1 {
			t.Errorf("失败渠道也应被调用 1 次，得到 %d", bad.callCount())
		}
		if good.callCount() != 1 {
			t.Errorf("正常渠道应照常发送 1 次，得到 %d（说明被失败渠道拖累了）", good.callCount())
		}
	})

	t.Run("未注册渠道_跳过_不影响已注册渠道", func(t *testing.T) {
		good := &mockNotifier{name: "system"}
		reg := NewRegistry()
		reg.Register(good)

		// "bark" 没注册，应被跳过并 log，"system" 照常发
		Broadcast(ctx, reg, []string{"bark", "system"}, msg)

		if good.callCount() != 1 {
			t.Errorf("已注册渠道应发送 1 次，得到 %d", good.callCount())
		}
	})

	t.Run("空渠道列表_不panic", func(t *testing.T) {
		reg := NewRegistry()
		Broadcast(ctx, reg, nil, msg) // 不应阻塞或 panic
	})
}
