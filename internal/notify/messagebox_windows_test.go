//go:build windows

package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSendWithCanceledContextSkipsDelivery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// 已经取消的 ctx 必须在调用任何 WTS 接口之前返回，避免真的弹窗。
	if err := (MessageboxNotifier{}).Send(ctx, Message{Title: "t", Body: "b"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("a\x00b", maxBodyRunes); got != "ab" {
		t.Fatalf("NUL not stripped: %q", got)
	}
	if got := truncateRunes("短消息", maxBodyRunes); got != "短消息" {
		t.Fatalf("short text changed: %q", got)
	}
	// 中文按字符截断，不能切在 UTF-8 字节中间。
	long := strings.Repeat("提", maxBodyRunes+10)
	got := truncateRunes(long, maxBodyRunes)
	if want := strings.Repeat("提", maxBodyRunes) + "…"; got != want {
		t.Fatalf("truncated = %q (len %d), want len %d with ellipsis", got, len([]rune(got)), maxBodyRunes+1)
	}
}

func TestInteractiveSessionID(t *testing.T) {
	id, err := interactiveSessionID()
	if errors.Is(err, ErrNoActiveSession) {
		t.Skip("当前没有可交互会话（例如无头会话中运行测试）")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("消息框将投递到会话 %d", id)
}
