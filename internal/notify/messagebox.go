package notify

import (
	"context"
	"errors"
)

// MessageboxNotifier 显示一条模态消息框。
//
// Send 只负责渠道语义（ctx 取消、取标题正文），"怎么在这个平台上把框弹出来"交给平台文件里的
// showDesktopMessage：
//   - messagebox_windows.go：WTSSendMessageW，可跨会话投递到用户桌面（服务在 session 0 也能弹），
//     投递后立即返回，不占用通知 worker
//   - messagebox_unix.go：dlgs，需要图形会话，Linux 上依赖 qarma 或 zenity，会阻塞到用户关闭
type MessageboxNotifier struct{}

func (messagebox MessageboxNotifier) Name() string {
	return "messagebox"
}

func (messagebox MessageboxNotifier) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return showDesktopMessage(message.Title, message.Body)
}

func (messagebox MessageboxNotifier) Close() error {
	return nil
}

// ErrNoActiveSession 表示当前没有可交互的用户会话（例如无人登录、纯无头机器）。
// 桌面类渠道可以据此跳过投递，而不是把消息记成发送失败。
var ErrNoActiveSession = errors.New("notify: 当前没有可交互的用户会话")
