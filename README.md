# Butler 电子管家

一个自托管的**定时通知 + 项目计划跟踪**工具。一棵 Node 树同时管理生日提醒（农历/公历）和项目里程碑，到点把消息并发推送到多个可插拔渠道。Go 单 binary，常驻 VPS 调度。

- 主链路：`mqtt`（发布到外置 Mosquitto，自有 MAUI 安卓端订阅弹通知）
- 兜底：`email`（SMTP）
- 本地开发验收：`system`（beeep 弹 Windows toast）

> 设计总览、核心抽象与开发阶段见 [DEVELOPMENT.md](./DEVELOPMENT.md)。
> 人与 Claude 的协作约定见 [CLAUDE.md](./CLAUDE.md)。

## 配置与文件布局

Butler 用两个 JSONC 文件（`github.com/tailscale/hujson` 解析，支持注释和尾逗号），职责分开：

| 文件 | 位置 | 内容 | 进 git？ |
|---|---|---|---|
| `plan.jsonc` | 项目文件夹（或 `--plan` 指定） | 计划树：生日、里程碑、调度、通知偏移 —— **唯一真相源**，AI 直接编辑 | ❌ 进 .gitignore（含生日等隐私） |
| `config.jsonc` | `os.UserConfigDir()/butler/` | 渠道密钥与连接信息（SMTP、MQTT broker） | ❌ 永不进 git |

> `plan.jsonc` 留在项目文件夹是为了让 AI 在工作区内直接读写；进 `.gitignore` 是因为它含生日等隐私，不希望被提交。代价是放弃 git 历史回滚——改坏由 `butler validate` 兜底。

**`os.UserConfigDir()` 的实际位置：**
- Windows → `%AppData%\butler\config.jsonc`
- Linux（VPS）→ `~/.config/butler/config.jsonc`

计划树里的节点只**引用渠道名**（如 `"channels": ["mqtt", "email"]`），密钥只存在于 `config.jsonc`，永不出现在真相源里——这样 AI 编辑计划时不会碰到任何密钥。

### 路径解析优先级

两个文件的位置都可被覆盖，方便部署到不同环境（如 VPS systemd 服务）：

```
plan.jsonc:    --plan / -p   >  BUTLER_PLAN    >  项目目录/plan.jsonc
config.jsonc:  --config / -c >  BUTLER_CONFIG  >  os.UserConfigDir()/butler/config.jsonc
```

> ⚠️ `plan.jsonc` 与 `config.jsonc` 都已写入 `.gitignore`，绝不进 git；建议 `config.jsonc` 文件权限设为 `0600`。密钥绝不硬编码进源码或 git。

## 快速开始

> 早期开发中，当前已跑通：本地 `system` 通知 + 一次性（Once）调度闭环。

```sh
# 构建
go build -o butler .

# 前台运行调度器
./butler serve

# 查看命令
./butler --help
```

## 技术栈

| 用途 | 选型 |
|---|---|
| CLI | `github.com/spf13/cobra` |
| JSONC 解析 | `github.com/tailscale/hujson` |
| cron 解析 | `github.com/robfig/cron/v3` |
| 系统通知 | `github.com/gen2brain/beeep` |
| 后台服务 | `github.com/kardianos/service` |
| 农历 | `github.com/6tail/lunar-go` |
| TUI（可选） | `github.com/charmbracelet/bubbletea` |
