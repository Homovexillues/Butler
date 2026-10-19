# Butler 电子管家

Butler 是一个自托管的定时任务与多渠道通知工具。任务和执行状态保存在 SQLite 中，可通过 HTTP API 创建、查询、修改和删除；程序按调度配置发送提醒，也可以先执行外部命令，再发送执行结果。

当前功能：

- 调度：一次性时间、公历周年、农历周年、五字段 Cron、固定间隔及触发偏移。
- 动作：普通提醒，或执行带参数、工作目录、环境变量的外部命令。
- 通知：系统桌面通知、消息框、邮件、MQTT。
- MQTT：进程内运行 Mochi Broker，支持 TLS、用户名密码认证和通知主题订阅。
- CLI：前台运行、任务列表、配置及任务校验、渠道测试、服务安装和更新。
- HTTP：任务 CRUD，以及适用于手机和 PC 的嵌入式管理页面，支持搜索、新建、编辑和删除任务。

旧版 `PlanNode` / `Children` 计划树已不再作为任务来源，`serve` 不读取 `plan.jsonc`。仓库中的 [SCHEMA.md](./SCHEMA.md) 描述的是旧版计划格式，不适用于当前 Task API；当前字段以 [Task 定义](./internal/model/task.go) 和下文为准。

本文示例全部使用虚构内容及占位凭据，不包含实际账户或任务数据。

## 构建与测试

`go.mod` 要求 Go 1.26.2 或更新版本。数据层使用 GORM 和 `github.com/glebarez/sqlite` 纯 Go SQLite 驱动，可设置 `CGO_ENABLED=0` 构建，无需 SQLite 的 C 编译工具链。JSON 字段通过模型上的 `serializer:json` 标签自动读写。

```sh
go build -trimpath -tags=nomsgpack -ldflags="-s -w" -o butler .
./butler --help

go test ./...
go test -race ./...
go vet ./...
```

`nomsgpack` 用于排除项目未使用的 Gin MsgPack 支持，保留 JSON 和 HTML 接口。仓库中的 `build.fish` 也是构建入口，但当前脚本不带此标签。

Windows 可使用 `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags nomsgpack -o butler.exe .` 交叉编译。

## 配置与数据目录

程序使用运行账户的 `os.UserConfigDir()`，在其中建立 `butler` 子目录：

| 系统 | 默认目录 |
|---|---|
| Linux | `~/.config/butler/`；设置 `XDG_CONFIG_HOME` 后使用 `$XDG_CONFIG_HOME/butler/` |
| Windows | `%AppData%\butler\` |

| 文件 | 用途 |
|---|---|
| `config.jsonc` | MQTT Broker、SMTP 等渠道配置 |
| `butler.db` | SQLite 任务配置与 `LastFired` 执行状态 |
| `butler.db-wal` / `butler.db-shm` | SQLite WAL 模式运行时可能产生的辅助文件 |
| `logs/YYYY-MM-DD.log` | `serve` 的 JSON 日志，当前保留 7 天 |

首次加载时会创建缺失的配置文件，但内容为空，仍需手动填写有效 JSON。`config.jsonc` 支持注释和尾逗号；数据库由程序初始化和迁移。

服务账户决定配置和数据目录。交互运行和系统服务如果使用不同账户，可能读取不同数据库。备份运行中的 SQLite 时应采用一致性备份方式，不要只复制主文件而忽略尚未合并的 WAL 数据。

### 最小配置

只使用本地桌面渠道时，可先使用：

```json
{
  "mqtt": {},
  "email": {}
}
```

当前启动流程仍会尝试创建 MQTT 和邮件渠道，因此未填写的渠道会记录初始化失败，但不影响其他成功创建的渠道。任务只能使用已定义的渠道名称，实际发送还要求该渠道可用。

### MQTT 与邮件配置

```json
{
  "mqtt": {
    "address": "127.0.0.1:8883",
    "topic": "butler/notifications",
    "username": "example-client",
    "password": "REPLACE_WITH_A_STRONG_PASSWORD",
    "certfile": "/etc/butler/tls/server.crt",
    "keyfile": "/etc/butler/tls/server.key",
    "skipverify": false
  },
  "email": {
    "host": "smtp.example.com",
    "port": 465,
    "username": "sender@example.com",
    "authcode": "REPLACE_WITH_SMTP_CREDENTIAL",
    "from": "sender@example.com",
    "to": ["recipient@example.com"]
  }
}
```

MQTT 配置是**本程序 Broker 的监听配置**，不是外部 Broker 地址：

- `address` 只填监听地址和端口，不带 `tcp://` 或 `tls://`。`127.0.0.1` 仅供本机访问。
- 当前实现要求 `address`、`topic`、`username`、`password` 非空。
- `skipverify: false` 时加载服务端证书及匹配私钥；证书不会自动生成。自签名证书需要客户端显式信任，证书域名/IP 应匹配客户端连接地址。
- **当前 `skipverify: true` 实际表示关闭 TLS、使用明文 TCP**，并非“加密但跳过验证”。这是现有字段命名与实现之间的差异。
- `certfile` 的相对路径以 Butler 配置目录为基准；`keyfile` 当前没有相同处理，建议使用绝对路径。
- 外部客户端使用配置的账户订阅指定主题；该账户当前只获得该主题的读取权限。Butler 通过进程内发布，不需要再连接自己的 Broker。
- `clientId` 字段仍存在于配置结构中，但当前 Broker 模式不使用它。

MQTT 消息是包含 `Title`、`Body` 的 JSON。发布成功表示 Broker 接受消息，不代表订阅设备已经收到或展示。当前未配置 Broker 持久化钩子，不能依赖它在重启后恢复离线通知。

邮件使用 SMTP SSL 连接，示例端口为 465。请按邮件服务提供方的要求填写账户及授权凭据。

## 快速开始

1. 在配置目录创建有效的 `config.jsonc`。
2. 执行 `./butler validate` 检查配置与数据库任务。
3. 执行 `./butler serve` 启动服务。
4. 通过下面的 HTTP API 添加任务。
5. 使用 `./butler tree` 查看下一次计划时间。

`serve` 固定监听 HTTP `:8191`。当前 API 尚未接入认证，包括可配置外部命令的写接口；请限制访问范围，不要直接开放到不受信任的网络。

## Task 配置

Task 是平面任务，不再区分父分组和叶子节点，也没有渠道继承。

| JSON 字段 | 说明 |
|---|---|
| `id` | 数据库 ID；新增任务时省略 |
| `title` / `body` | 通知标题与正文 |
| `scheduleType` | `once`、`solar`、`lunar`、`cron`、`interval` |
| `scheduleExpression` | 对应类型的调度表达式 |
| `scheduleOffset` | 偏移数组，如 `["T-1d", "T-0d"]`；无需偏移时使用 `[]` |
| `actionType` | `""` 表示普通提醒；`"command"` 表示执行外部命令 |
| `actionExpression` | 动作 JSON 对象；普通提醒填写 `{}`，不要传空字符串 |
| `channel` | 渠道数组，例如 `["system", "mqtt"]`；注意 Task 的字段名是单数 `channel` |
| `enabled` | 是否启用；省略时为 false |
| `lastFired` | 上次成功执行完成的时间，通常由服务维护，创建时省略 |

### 调度表达式

| 类型 | 示例 | 说明 |
|---|---|---|
| `once` | `2030-06-01 09:00:00` | 当前使用 `time.Parse`，按 UTC 解析 |
| `solar` | `06-01 09:00:00` | 每年公历月日，触发时间使用本地时区 |
| `lunar` | `01-01 09:00:00` | 每年农历月日，换算后的触发时间使用本地时区 |
| `cron` | `0 9 * * 1-5` | 五字段：分、时、日、月、星期；不包含秒 |
| `interval` | `1h,30m` | 整数片段以逗号分隔，支持 `d`、`h`、`m`、`s`；不是 `1h30m` |

偏移支持 `T-1d`、`T-2h`、`T-30m`、`T+1h` 等形式。非空偏移数组会替代基础触发时间集合；若同时需要基础时刻，需显式包含 `T-0d`。偏移格式的严格校验仍不完整。

### 普通提醒

保存为 `task.json`：

```json
{
  "title": "工作日提醒",
  "body": "起身活动一下",
  "scheduleType": "cron",
  "scheduleExpression": "0 10 * * 1-5",
  "scheduleOffset": [],
  "actionType": "",
  "actionExpression": {},
  "channel": ["system"],
  "enabled": true
}
```

### 外部命令任务

`actionType` 改为 `command`，`actionExpression` 使用：

```json
{
  "Command": "example-tool",
  "Args": ["--check"],
  "Dir": "/srv/example",
  "Env": {
    "EXAMPLE_MODE": "scheduled"
  }
}
```

请将示例程序和目录替换为实际值。命令直接执行，不经过 shell；管道、重定向和 shell 变量展开不会自动生效。命令以 Butler 进程的账户权限运行。执行输出或错误会附加到任务消息，并使用任务的通知渠道发送。

## HTTP API

| 方法与路径 | 用途 | 请求内容 |
|---|---|---|
| `GET /` | 嵌入式任务管理页面 | 无 |
| `GET /ping` | 返回 `pong` | 无 |
| `GET /api/v1/tasks` | 任务列表（含完整配置及下次触发时间） | 无 |
| `POST /api/v1/add` | 创建 Task | Task 对象，省略 ID |
| `POST /api/v1/get` | 查询完整 Task | `{"id": 1}` |
| `POST /api/v1/update` | 更新 Task 配置 | 含 ID 的完整 Task 对象 |
| `POST /api/v1/delete` | 删除 Task | `{"id": 1}` |

旧版 `/healthz` 已移除。POST 请求使用 `Content-Type: application/json`。

```sh
# 创建任务；响应中的 id 用于后续查询、修改、删除
curl -sS http://127.0.0.1:8191/api/v1/add \
  -H 'Content-Type: application/json' \
  --data-binary @task.json

# 将 1 替换为创建接口返回的 ID
curl -sS http://127.0.0.1:8191/api/v1/get \
  -H 'Content-Type: application/json' \
  -d '{"id":1}'

curl -sS http://127.0.0.1:8191/api/v1/tasks
```

修改时先获取完整 Task，更改需要的字段，再连同 ID 提交到 `/update`。当前更新不是 PATCH，省略字段可能将其清空或重置。`LastFired` 不属于普通配置更新字段。

服务进程内经 CRUD 成功写入的变更会通知 engine 重新加载任务；直接用其他进程修改 SQLite 不会触发这个内存通道。`config.jsonc` 的修改需要重启才能生效。

`/tasks` 返回完整 Task 字段及 `nextTriggeredAt`，并保留复数 `channels` 供列表客户端使用；提交 Task 仍使用单数 `channel`。停用或无后续触发时间时，`nextTriggeredAt` 为 `null`；调度配置解析失败时，该任务携带 `scheduleError`，不影响其他任务显示。

## CLI

| 命令 | 行为 |
|---|---|
| `serve` | 前台运行，或作为已注册服务的启动入口 |
| `tree` | 查库并输出任务列表；以当前时间计算下一次计划时间，按时间升序排列，禁用任务也显示计划时间 |
| `validate` | 汇总配置校验及全部 Task 转 Node 的错误，包括禁用任务；不执行动作或发送通知 |
| `test <channel>` | 立即测试一个通知渠道 |
| `install [--user USER]` | 复制可执行文件到安装位置，并注册服务；不自动启动 |
| `start` / `stop` | 启动或停止已注册服务 |
| `uninstall` | 取消服务注册，不等同于删除配置、数据库或安装文件 |
| `update` | 用当前运行的可执行文件覆盖安装文件；更新前运行的服务会被停止，并在复制成功后重新启动 |
| `node <crud>` | 尚未实现，仅为占位命令 |

`system` 会蜂鸣并发送桌面通知；`messagebox` 会等待消息框退出。Linux 桌面渠道依赖可用的图形会话，消息框还需 `qarma` 或 `zenity`。系统服务环境不等同于登录桌面，注册成功不代表桌面通知可用。

`test` 会初始化通知渠道；测试 MQTT 会尝试启动本地 Broker，可能与正在运行的服务发生端口冲突。

### 安装与更新

| 平台 | 可执行文件安装位置 |
|---|---|
| Linux | `/usr/local/bin/butler` |
| Windows | `%ProgramFiles%\Butler\butler.exe` |

Linux 示例：

```sh
# 注册并指定运行账户；账户决定配置和数据库目录
sudo ./butler install --user "$(id -un)"
sudo ./butler start

# 从安装目录外的新版本程序执行更新
sudo ./butler update

sudo ./butler stop
sudo ./butler uninstall
```

安装和覆盖系统目录需要相应权限。更新应明确使用 `./butler` 指向新构建文件；`butler update` 可能运行 PATH 中的旧版本。

当前 `update` 保留更新前的运行状态：如果服务已经停止，更新后不会自动启动，需手动执行 `start`。更新采用直接覆盖，尚无备份回滚及完整的跨平台停止等待机制；替换失败可能留下停止的服务或不完整文件。安装命令也不应用作升级命令。

Windows 使用系统服务 API，默认不询问 Linux 运行用户；指定普通 Windows 服务账户的密码及权限流程尚不完整。Windows 服务与登录桌面隔离，桌面通知场景需要额外设计。当前安装路径选择只实现 Linux 和 Windows。

## 调度与通知语义

- engine 按任务 ID 跟踪运行状态，同一个任务正在执行时不会再次启动它。
- 从未执行过的任务以加载时刻作为内存起算时间；这个起点尚未持久化，首次执行前重启可能重置间隔等待。
- 已有 `LastFired` 的任务从该时间寻找下一次；若计划时间已过，会立即尝试执行。成功后记录完成时间，因此 Interval 表示“成功完成后再等一个间隔”。
- 命令失败不推进 `LastFired`，当前没有重试退避，可能快速重复执行并发送错误通知。
- 动作完成后将消息投递到通知队列，任务状态与通知送达状态不同；通知失败不会自动重新执行已成功的命令，通知队列也没有持久化。
- 指定渠道并发发送；失败报告可能走其他渠道。整体广播成功只代表至少一个渠道发送成功，可能是备用失败报告成功，而非原渠道成功。
- `tree` 是基于当前时间的计划预览，不是 engine 内存状态查询。已有历史的 Interval 预览时间可能与 engine 实际安排不同。

## 当前限制

- 管理页面的下次触发时间由任务配置和上次执行时间计算；从未执行的间隔任务以查询时刻估算，不是 engine 运行状态的实时快照。
- 调度、动作、偏移校验尚不覆盖全部业务错误；`validate` 通过不代表外部命令存在、证书可用或通知已送达。
- 农历表达式当前借用公历月日解析，部分合法农历日期尚不能正确表达。
- 通知渠道阻塞可能耗尽 worker，并在队列满时拖慢调度；退出时的后台任务等待和资源清理仍需完善。

## 技术栈

| 用途 | 依赖 |
|---|---|
| CLI / 系统服务 | Cobra、kardianos/service |
| 数据存储 | GORM、SQLite、glebarez/sqlite（纯 Go，无 CGO） |
| 调度 | robfig/cron、lunar-go |
| HTTP / 页面嵌入 | Gin、Go embed |
| MQTT Broker | mochi-mqtt/server |
| 邮件与桌面通知 | go-mail、beeep、dlgs |
| JSONC / 日志 | hujson、file-rotatelogs、log/slog |
