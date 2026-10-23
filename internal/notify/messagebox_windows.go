//go:build windows

package notify

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// Windows 下 messagebox 渠道走 WTSSendMessageW。
//
// 系统服务运行在 session 0，那里没有可见桌面：user32!MessageBoxW 会把窗口画在服务的隐藏
// 窗口站上，并一直等待一个没人能点的确认按钮，通知 worker 因此永久挂住。WTSSendMessageW
// 是 Windows 官方提供的跨会话投递接口，可以把消息框显示到指定会话（通常是控制台会话）的
// 默认桌面上，服务账户也能调用。
//
// 这里的取舍：
//   - bWait = FALSE：投递后立刻返回，绝不阻塞通知 worker。代价是拿不到用户的选择，而本渠道
//     本来也不需要它。
//   - Timeout：给消息框一个有限生命周期，避免桌面上堆积没人处理的窗口。
//   - 会话选择：优先控制台会话；没有控制台会话（纯 RDP、无头机器）时退到任意活动会话；
//     都没有则返回 ErrNoActiveSession，交给调用方决定是跳过还是报告。

const (
	wtsCurrentServerHandle = 0          // WTS_CURRENT_SERVER_HANDLE
	wtsActive              = 0          // WTS_CONNECTSTATE_CLASS: WTSActive
	wtsNoSession           = 0xFFFFFFFF // WTSGetActiveConsoleSessionId 无控制台会话时的返回值

	mbOK              = 0x00000000 // MB_OK
	mbIconInformation = 0x00000040 // MB_ICONINFORMATION
	mbTopmost         = 0x00040000 // MB_TOPMOST：提醒要能看见，但不抢焦点

	// messageBoxTimeoutSeconds 为 0 表示永不超时；给有限值避免窗口长期滞留。
	messageBoxTimeoutSeconds = 20

	// WTSSendMessage 走 WTS RPC，正文过长容易被拒绝或被窗口截断；动作输出会拼进正文，
	// 这里做兜底截断。
	maxTitleRunes = 120
	maxBodyRunes  = 1024
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	wtsapi32 = syscall.NewLazyDLL("wtsapi32.dll")

	// WTSGetActiveConsoleSessionId 由 Kernel32.dll 导出（头文件是 winbase.h），
	// 不在 wtsapi32.dll 里 —— 从 wtsapi32 取它会得到"找不到过程"的 panic。
	procWTSGetActiveConsoleSessionId = kernel32.NewProc("WTSGetActiveConsoleSessionId")
	procWTSEnumerateSessionsW        = wtsapi32.NewProc("WTSEnumerateSessionsW")
	procWTSFreeMemory                = wtsapi32.NewProc("WTSFreeMemory")
	procWTSSendMessageW              = wtsapi32.NewProc("WTSSendMessageW")
)

// resolve 把"系统缺少某个导出函数"从 panic 变成错误。
// syscall.LazyProc.Call 在找不到符号时会直接 panic，而通知渠道失败不应该打死整个进程。
func resolve(procs ...*syscall.LazyProc) error {
	for _, proc := range procs {
		if err := proc.Find(); err != nil {
			return fmt.Errorf("notify: 系统缺少 %s 导出: %w", proc.Name, err)
		}
	}
	return nil
}

// wtsSessionInfoW 对应 WTS_SESSION_INFOW；Go 会为 *uint16 自动补齐对齐，布局与 C 一致。
type wtsSessionInfoW struct {
	SessionID      uint32
	WinStationName *uint16
	State          uint32
}

// showDesktopMessage 把消息框投递到用户会话的桌面，投递后立即返回（bWait = FALSE）。
func showDesktopMessage(title, body string) error {
	if err := resolve(procWTSGetActiveConsoleSessionId, procWTSEnumerateSessionsW,
		procWTSFreeMemory, procWTSSendMessageW); err != nil {
		return err
	}

	sessionID, err := interactiveSessionID()
	if err != nil {
		return err
	}

	title = truncateRunes(title, maxTitleRunes)
	body = truncateRunes(body, maxBodyRunes)
	if err := sendToSession(sessionID, title, body); err != nil {
		return fmt.Errorf("messagebox: fail to deliver to session %d: %w", sessionID, err)
	}
	return nil
}

// interactiveSessionID 返回消息框应该投递到的会话。
func interactiveSessionID() (uint32, error) {
	if id, _, _ := procWTSGetActiveConsoleSessionId.Call(); uint32(id) != wtsNoSession {
		return uint32(id), nil
	}
	// 没有控制台会话：纯 RDP 登录、无头机器或没有用户登录。
	if id, ok := firstActiveSession(); ok {
		return id, nil
	}
	return 0, ErrNoActiveSession
}

// firstActiveSession 枚举所有会话，返回第一个处于活动状态的会话。
func firstActiveSession() (uint32, bool) {
	var (
		sessions *wtsSessionInfoW
		count    uint32
	)
	// BOOL WTSEnumerateSessionsW(HANDLE hServer, DWORD Reserved, DWORD Version,
	//     PWTS_SESSION_INFO *ppSessionInfo, DWORD *pCount)；Version 必须为 1。
	ok, _, _ := procWTSEnumerateSessionsW.Call(
		wtsCurrentServerHandle,
		0,
		1,
		uintptr(unsafe.Pointer(&sessions)),
		uintptr(unsafe.Pointer(&count)),
	)
	if ok == 0 || sessions == nil || count == 0 {
		return 0, false
	}
	defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(sessions)))

	for _, session := range unsafe.Slice(sessions, count) {
		if session.State == wtsActive {
			return session.SessionID, true
		}
	}
	return 0, false
}

func sendToSession(sessionID uint32, title, body string) error {
	title16, err := syscall.UTF16FromString(title)
	if err != nil {
		return err
	}
	body16, err := syscall.UTF16FromString(body)
	if err != nil {
		return err
	}

	var response uint32
	// BOOL WTSSendMessageW(HANDLE hServer, DWORD SessionId, LPWSTR pTitle, DWORD TitleLength,
	//     LPWSTR pMessage, DWORD MessageLength, DWORD Style, DWORD Timeout,
	//     DWORD *pResponse, BOOL bWait)
	// 两个 Length 的单位是字节，不是字符数。
	ok, _, callErr := procWTSSendMessageW.Call(
		wtsCurrentServerHandle,
		uintptr(sessionID),
		uintptr(unsafe.Pointer(&title16[0])),
		uintptr(len(title16)-1)*2,
		uintptr(unsafe.Pointer(&body16[0])),
		uintptr(len(body16)-1)*2,
		mbOK|mbIconInformation|mbTopmost,
		messageBoxTimeoutSeconds,
		uintptr(unsafe.Pointer(&response)),
		0, // bWait = FALSE
	)
	if ok != 0 {
		return nil
	}

	errno, _ := callErr.(syscall.Errno)
	switch errno {
	case 0:
		return errors.New("WTSSendMessageW returned FALSE without an error code")
	case syscall.ERROR_ACCESS_DENIED:
		return fmt.Errorf("%w；服务账户需要能向该会话投递消息，LocalSystem 或具备相应权限的账户通常可以", errno)
	default:
		return errno
	}
}

// truncateRunes 去掉 NUL（UTF16FromString 会拒绝它）并限制长度。
func truncateRunes(s string, maxRunes int) string {
	s = strings.ReplaceAll(s, "\x00", "")
	// 字节数不少于字符数，可以先做一次便宜的判断。
	if len(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}
