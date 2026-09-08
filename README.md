# Butler 电子管家

Butler 是一个自托管的定时调度与通知工具。它从一棵 JSONC 计划树中加载任务，按照一次性日期、公历/农历周年、Cron 或固定间隔触发通知和命令。

当前实现适合以单个 Go 可执行文件常驻运行：

- 调度：`Once`、`Solar`、`Lunar`、`Cron`、`Interval`
- 动作：多渠道通知、执行外部命令
- 渠道：`system`、`messagebox`、`mqtt`、`email`
- 状态：任务成功后把 `LastFired` 写回计划文件
- 服务：前台运行，或在 Linux 上注册为 systemd 服务
- HTTP：提供健康检查和只读任务列表

计划文件的完整字段和约束见 [SCHEMA.md](./SCHEMA.md)。

## 构建

`go.mod` 当前要求 Go 1.26.2：

```sh
go build -o butler .
./butler --help
```

## 配置文件

Butler 使用 `os.UserConfigDir()`，并在其下建立 `butler` 子目录：

| 系统 | 默认目录 |
|---|---|
| Linux | `~/.config/butler/`，设置了 `XDG_CONFIG_HOME` 时使用 `$XDG_CONFIG_HOME/butler/` |
| Windows | `%AppData%\butler\` |
| macOS | `~/Library/Application Support/butler/` |

目录中的文件为：

| 文件 | 用途 | 创建权限 |
|---|---|---|
| `plan.jsonc` | 计划树、动作以及运行状态 | `0644` |
| `config.jsonc` | MQTT、SMTP 连接信息和密钥 | `0600` |
| `logs/YYYY-MM-DD.log` | `serve` 的 JSON 日志，保留 7 天 | `0755` 日志目录 |

缺少的配置文件会被创建为空文件，但空文件不能被解析；第一次运行前仍需写入有效内容。

两个配置文件都使用 JSONC 解析，读取时允许注释和尾逗号。需要特别注意：任务成功后，Butler 为保存 `LastFired` 会原子重写整个 `plan.jsonc`；重写结果是标准 JSON，原有注释和排版不会保留。

### config.jsonc

未使用的渠道可以保留为空对象。`broker` 只填写 `主机:端口`，不要带 `tcp://` 或 `tls://`；配置 `certfile` 或将 `skipverify` 设为 `true` 时使用 TLS。相对形式的 `certfile` 从 Butler 配置目录解析。

```jsonc
{
  "mqtt": {
    "broker": "mqtt.example.com:8883",
    "topic": "butler/notify",
    "clientId": "butler-server",
    "username": "butler",
    "password": "change-me",
    "certfile": "mqtt-ca.crt",
    "skipverify": false
  },
  "email": {
    "host": "smtp.example.com",
    "port": 465,
    "username": "butler@example.com",
    "authcode": "change-me",
    "from": "butler@example.com",
    "to": ["me@example.com"]
  }
}
```

如果只使用本机的 `system` 或 `messagebox` 渠道，可以写成：

```json
{
  "mqtt": {},
  "email": {}
}
```

### plan.jsonc

每个叶子任务配置一个调度和一个动作。通知渠道写在 `NotifyAction.Channels` 中；当前版本不会从分组节点继承渠道。

```jsonc
{
  "Children": [
    {
      "Title": "日常提醒",
      "Children": [
        {
          "Title": "喝水",
          "Body": "起来活动一下，顺便喝水",
          "Interval": "15m",
          "NotifyAction": {
            "Channels": ["mqtt", "system"]
          }
        },
        {
          "Title": "下班打卡",
          "Body": "记得打卡",
          "Cron": "10 18 * * 1-5",
          "NotifyAction": {
            "Channels": ["mqtt"]
          }
        }
      ]
    },
    {
      "Title": "家人生日",
      "Body": "提前准备礼物",
      "Lunar": "01-29 09:00:00",
      "TriggerOffset": ["T-3d", "T-0d"],
      "NotifyAction": {
        "Channels": ["mqtt", "email"]
      }
    }
  ]
}
```

## 使用

```sh
# 同时检查 config.jsonc 和 plan.jsonc
./butler validate

# 预览任务树和下一触发时间
./butler tree

# 立即测试一个渠道
./butler test system

# 前台启动调度器
./butler serve
```

`test` 支持 `system`、`messagebox`、`mqtt` 和 `email`。桌面渠道需要图形会话；在无桌面的服务器上通常应使用 MQTT 或邮件。MQTT 采用异步连接，刚启动的 `butler test mqtt` 可能在连接建立前返回 `mqtt broker not connected`；常驻的 `serve` 会自动重连。

`serve` 启动时加载一次配置和计划，当前不支持热重载；修改文件后需要重启进程。

### Linux 服务

```sh
# 注册时明确指定服务运行用户；该用户决定读取哪一套配置文件
sudo ./butler install --user "$(id -un)"
sudo ./butler start

sudo ./butler stop
sudo ./butler uninstall
```

服务运行用户必须能够读取 `config.jsonc`，并且必须能够写入 `plan.jsonc`，否则成功执行后无法持久化 `LastFired`。

## 调度语义

- `Once`、`Solar`、`Lunar` 和 `Cron` 使用进程所在系统的本地时区。
- Interval 的设计目标是在没有 `LastFired` 时从启动时刻等待一个完整间隔。但当前引擎每分钟重新扫描，并在每次扫描时重新取 `time.Now()`；因此新建且大于 1 分钟的 Interval 可能被不断顺延。修复前可显式提供初始 `LastFired`，或者只把该行为用于测试。
- 任务成功后，`LastFired` 记录动作完成时刻；所以下一次 Interval 是“上次完成时间 + 间隔”，不是固定墙钟刻度。
- 动作失败时不会更新 `LastFired`。如果已有的非零 `LastFired` 使下一触发时间仍然过期，引擎会立即重试，目前没有退避策略；首次执行失败时的行为取决于调度类型。
- `butler tree` 从当前时间计算预览值。对于已有 `LastFired` 的 Interval，调度器实际使用 `LastFired + Interval`，因此预览值可能与真实下一次触发时间不同。
- 同一条通知的多个渠道并发发送。指定渠道失败后，Butler 会尝试通过已成功渠道或其他已注册渠道发送失败报告；只要原通知或后续失败报告至少有一个渠道发送成功，该通知动作就会被视为成功。

## 动作

### NotifyAction

`Title` 和 `Body` 组成消息内容，`NotifyAction.Channels` 决定发送目标：

| 渠道 | 行为 |
|---|---|
| `system` | 蜂鸣并显示系统桌面通知 |
| `messagebox` | 显示模态消息框 |
| `mqtt` | 以 QoS 1 发布包含 `Title`、`Body` 的 JSON |
| `email` | 以 `Title` 为主题、`Body` 为正文发送纯文本邮件 |

### CommandAction

`CommandAction` 直接启动程序，不经过 shell；支持参数、工作目录和附加环境变量。退出码非零时动作失败，合并后的 stdout/stderr 会写入错误。命令成功且有输出时，当前实现仅通过 `mqtt` 发送输出；通知发送失败只记日志，不会令命令动作失败。

```jsonc
{
  "Title": "每日备份",
  "Cron": "0 3 * * *",
  "CommandAction": {
    "Command": "/usr/local/bin/backup",
    "Args": ["--incremental"],
    "Dir": "/srv/app",
    "Env": {
      "BACKUP_MODE": "daily"
    }
  }
}
```

## HTTP 接口

`serve` 固定监听 `:8191`：

| 方法与路径 | 用途 |
|---|---|
| `GET /ping` | 返回 `pong` |
| `GET /healthz` | 返回 `{"status":"ok"}` |
| `GET /api/v1/tasks` | 返回启动时加载的叶子任务及基于当前时间计算的下一触发时间 |

接口目前只读，也没有认证；不要直接暴露到不受信任的网络。`node <crud>` 命令目前只是占位，尚未实现节点增删改查。

## 技术栈

| 用途 | 依赖 |
|---|---|
| CLI / 服务 | Cobra、kardianos/service |
| JSONC | hujson |
| Cron | robfig/cron |
| HTTP | Gin |
| MQTT | Eclipse Paho MQTT |
| 邮件 | go-mail |
| 桌面通知 | beeep、dlgs |
| 农历 | lunar-go |
| 日志轮转 | file-rotatelogs |
