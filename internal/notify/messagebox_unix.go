//go:build unix

package notify

import "github.com/gen2brain/dlgs"

// showDesktopMessage 在 Linux/macOS 上弹消息框：需要图形会话，Linux 上还依赖 qarma 或 zenity。
// 会阻塞到用户关闭窗口，因此会占用调用它的通知 worker。
//
// 注意 build tag：unix 不是 GOOS 值，文件名里的 _unix 后缀不构成隐式约束，
// 必须显式写 //go:build unix，否则 Windows 构建会把两个实现都编进来（重复声明）。
func showDesktopMessage(title, body string) error {
	_, err := dlgs.Info(title, body)
	return err
}
