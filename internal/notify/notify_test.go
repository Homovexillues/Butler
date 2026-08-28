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

	mu      sync.Mutex
	calls   int
	gotMsgs []Message
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

		if err := broadcast(ctx, reg, []string{"system", "mqtt"}, msg); err != nil {
			t.Fatalf("broadcast 返回错误：%v", err)
		}

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

		// mqtt 失败后，已经成功的 email 会再收到一次失败报告。
		// 只要失败报告成功送达，整个通知请求就视为成功。
		err := broadcast(ctx, reg, []string{"mqtt", "email"}, msg)
		if err != nil {
			t.Fatalf("已有渠道成功并收到失败报告，broadcast 不应返回错误：%v", err)
		}

		if bad.callCount() != 1 {
			t.Errorf("失败渠道也应被调用 1 次，得到 %d", bad.callCount())
		}
		if good.callCount() != 2 {
			t.Errorf("正常渠道应收到原消息和失败报告，共 2 次，得到 %d", good.callCount())
		}
		if len(good.gotMsgs) != 2 {
			t.Fatalf("email 收到 %d 条消息，期望 2 条", len(good.gotMsgs))
		}
		failureReport := good.gotMsgs[1]
		if failureReport.Title != msg.Title+"(含失败报告)" {
			t.Errorf("失败报告标题 = %q，期望 %q", failureReport.Title, msg.Title+"(含失败报告)")
		}
		if failureReport.Body == msg.Body {
			t.Error("失败报告正文没有包含渠道错误")
		}
	})

	t.Run("未注册渠道_跳过_不影响已注册渠道", func(t *testing.T) {
		good := &mockNotifier{name: "system"}
		reg := NewRegistry()
		reg.Register(good)

		// "bark" 没注册，但 system 能发送原消息和失败报告，
		// 因此整体通知仍然成功。
		if err := broadcast(ctx, reg, []string{"bark", "system"}, msg); err != nil {
			t.Fatalf("system 已成功接收失败报告，不应返回错误：%v", err)
		}

		if good.callCount() != 2 {
			t.Errorf("已注册渠道应收到原消息和失败报告，共 2 次，得到 %d", good.callCount())
		}
	})

	t.Run("原渠道全部失败_由备用渠道报告成功", func(t *testing.T) {
		bad := &mockNotifier{name: "mqtt", failWith: errors.New("broker 挂了")}
		fallback := &mockNotifier{name: "email"}
		reg := NewRegistry()
		reg.Register(bad)
		reg.Register(fallback)

		if err := broadcast(ctx, reg, []string{"mqtt"}, msg); err != nil {
			t.Fatalf("备用 email 已成功报告错误，不应返回错误：%v", err)
		}
		if bad.callCount() != 1 {
			t.Errorf("mqtt 被调用 %d 次，期望 1", bad.callCount())
		}
		if fallback.callCount() != 1 {
			t.Errorf("备用 email 被调用 %d 次，期望 1", fallback.callCount())
		}
	})

	t.Run("原渠道和备用渠道都不可用_返回错误", func(t *testing.T) {
		brokerErr := errors.New("broker 挂了")
		bad := &mockNotifier{name: "mqtt", failWith: brokerErr}
		reg := NewRegistry()
		reg.Register(bad)

		err := broadcast(ctx, reg, []string{"mqtt"}, msg)
		if !errors.Is(err, brokerErr) {
			t.Fatalf("broadcast 错误 = %v，期望包含 %v", err, brokerErr)
		}
	})

	t.Run("空渠道列表_不panic", func(t *testing.T) {
		reg := NewRegistry()
		if err := broadcast(ctx, reg, nil, msg); err != nil {
			t.Fatalf("空渠道列表不应返回错误：%v", err)
		}
	})
}
