# plan.jsonc 字段说明

`plan.jsonc` 是 Butler 的计划树和运行状态文件。本文档以 `internal/parser/plan_dto.go` 中的 `PlanNode` 为准。

文件由 hujson 读取，支持 JSONC 注释和尾逗号。字段名由 Go `encoding/json` 处理，大小写不敏感，但建议统一使用本文中的 PascalCase。未知字段会被静默忽略，因此旧字段 `Channels`、`NotifyOffset` 等不会自动转换，必须改成当前结构。

> Butler 会在任务成功后写入 `LastFired`，并将整个文件重写为标准 JSON。不要依赖计划文件中的注释或手工排版长期保留。

## 顶层结构

顶层只是一个容器，实际节点放在 `Children` 中：

```jsonc
{
  "Children": [
    // PlanNode ...
  ]
}
```

## PlanNode

| 字段 | 类型 | 说明 |
|---|---|---|
| `Title` | string | 节点标题；分组和叶子节点都必须填写 |
| `Body` | string | 通知正文；可为空 |
| `Once` | string | 一次性调度 |
| `Solar` | string | 每年公历调度 |
| `Lunar` | string | 每年农历调度 |
| `Cron` | string | 标准五段 Cron 调度 |
| `Interval` | string | 从上次成功完成时刻起计算的固定间隔 |
| `NotifyAction` | object | 多渠道通知动作 |
| `CommandAction` | object | 外部命令动作 |
| `TriggerOffset` | []string | 相对目标时间的触发偏移 |
| `Children` | []PlanNode | 子节点；存在子节点时本节点是分组 |
| `LastFired` | RFC3339 string | 最近一次成功完成时间，由 Butler 维护 |

## 节点约束

计划树包含分组节点和叶子节点：

- 分组节点：`Children` 非空，不配置调度或动作，只负责组织层级。
- 叶子节点：没有 `Children`，必须配置且仅配置一个调度，并配置且仅配置一个动作。
- 可选调度为 `Once`、`Solar`、`Lunar`、`Cron`、`Interval`，五选一。
- 可选动作为 `NotifyAction`、`CommandAction`，二选一。
- 当前版本不继承分组上的动作或通知渠道。每个通知叶子都必须单独填写 `NotifyAction.Channels`。

当前实现对“同时填写两个动作”的校验不完整，并会优先使用 `CommandAction`；配置文件仍应遵守二选一约束。

## 调度字段

### Once

格式为本地时间 `YYYY-MM-DD HH:mm:ss`：

```json
"Once": "2026-09-30 14:00:00"
```

只触发一次。启动时已经过期的 Once 不会补发。

### Solar

格式为 `MM-DD HH:mm:ss`，每年按系统本地时区触发：

```json
"Solar": "03-05 09:00:00"
```

如果配置 `02-29` 而目标年份不是闰年，当前实现回退到 `02-28`。

### Lunar

格式为 `MM-DD HH:mm:ss`，其中月、日是农历日期：

```json
"Lunar": "01-29 09:00:00"
```

当前结构没有单独的闰月标志。

### Cron

使用标准五段格式 `分 时 日 月 周`：

```json
"Cron": "10 18 * * 1-5"
```

这表示每周一至周五 18:10，采用系统本地时区。Cron 用于对齐墙钟刻度；“从上一次完成后再等一段时间”应使用 Interval。

### Interval

每一段由正整数和单位组成，单位支持：

| 单位 | 含义 |
|---|---|
| `d` | 24 小时 |
| `h` | 小时 |
| `m` | 分钟 |
| `s` | 秒 |

单段示例：

```json
"Interval": "15m"
```

多个单位用英文逗号相加，不要插入空格：

```json
"Interval": "1d,2h,30m"
```

Interval 的运行语义为：

1. `LastFired` 为空时，调度器以扫描当时的时间作为计算基准。
2. 成功后把动作完成时间写入 `LastFired`。
3. 下一次触发时间为 `LastFired + Interval`。
4. 失败时不更新 `LastFired`；如果已有的非零 `LastFired` 使下一触发时间仍然过期，引擎会立即重试，目前没有退避策略。

总时长必须大于零。当前解析器可能不会为所有拼写错误返回明确错误，因此应严格使用上述格式。

当前还有一个首次调度限制：引擎最长每分钟唤醒一次，而无 `LastFired` 的 Interval 会在每轮扫描时重新取当前时间。新建且大于 1 分钟的 Interval 因而可能被不断顺延，不能保证按设计完成首次触发。修复初始化基准前，可以显式提供一个初始 `LastFired`；`1m` 只适合验证当前路径，不代表更长间隔已经正确处理。

## 动作字段

### NotifyAction

```jsonc
"NotifyAction": {
  "Channels": ["mqtt", "email"]
}
```

消息标题和正文分别取同一节点的 `Title`、`Body`。`Channels` 不可为空，可选值为：

| 值 | 说明 |
|---|---|
| `system` | 蜂鸣并显示系统桌面通知 |
| `messagebox` | 显示模态消息框 |
| `mqtt` | 发布 JSON 消息到配置的 MQTT topic |
| `email` | 发送纯文本邮件 |

各渠道需要在运行环境中可用。MQTT 和 email 的连接配置位于 `config.jsonc`，不应写入计划文件。

### CommandAction

```jsonc
"CommandAction": {
  "Command": "/usr/local/bin/backup",
  "Args": ["--incremental"],
  "Dir": "/srv/app",
  "Env": {
    "BACKUP_MODE": "daily"
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `Command` | string | 可执行程序路径或可从 `PATH` 查找的程序名，必填 |
| `Args` | []string | 直接传给程序的参数，不经过 shell 展开 |
| `Dir` | string | 可选工作目录 |
| `Env` | object<string,string> | 在现有进程环境上追加或覆盖的环境变量 |

行为说明：

- Butler 使用 `exec.CommandContext` 直接启动程序，管道、重定向、通配符和 shell 内建命令不会自动生效。确需 shell 时，应显式配置 shell 程序及其参数。
- stdout 和 stderr 会合并采集。退出码非零时动作失败，并在错误中保留输出。
- 命令成功且输出非空时，当前实现固定通过 `mqtt` 渠道发送输出。
- 命令输出的 MQTT 通知失败只会记录日志，不会把已经成功的命令改判为失败。

## TriggerOffset

`TriggerOffset` 会把基础调度转换为一个或多个相对触发时间。支持 `d`、`h`、`m`：

```jsonc
"TriggerOffset": ["T-3d", "T-2h", "T-0m"]
```

- 负数表示提前，例如 `T-3d` 是目标前三天。
- 零表示目标时刻，例如 `T-0d`、`T-0m`。
- 正数表示延后，例如 `T+30m` 是目标后三十分钟。
- 配置该字段后，只会生成列出的偏移；不会自动补充目标时刻。需要目标时刻提醒时必须显式加入零偏移。

当前解析器可能把某些残缺表达式当成零偏移，建议只使用 `T<有符号整数><单位>` 形式。

## LastFired

通常不要手工填写 `LastFired`。任务动作被判定成功后，Butler 会写入带时区的 RFC3339 时间，例如：

```json
"LastFired": "2026-09-08T14:50:11.4742631+08:00"
```

它主要用于计算重复任务的下一次触发时间。删除该字段会让节点回到“从未成功执行”的状态；对于 Interval，还会重新暴露上一节所述的首次调度限制。

`butler tree` 和只读 HTTP 接口目前基于当前时间调用调度器，不使用 `LastFired`。因此它们对 Interval 显示的是“当前时间 + 间隔”的预览，不一定等于调度引擎实际采用的 `LastFired + 间隔`。

## 完整示例

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
          "Title": "护眼",
          "Body": "看向远处，休息眼睛",
          "Interval": "20m",
          "NotifyAction": {
            "Channels": ["mqtt"]
          }
        }
      ]
    },
    {
      "Title": "纪念日",
      "Body": "提前准备",
      "Solar": "09-30 09:00:00",
      "TriggerOffset": ["T-3d", "T-0d"],
      "NotifyAction": {
        "Channels": ["mqtt", "email"]
      }
    },
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
  ]
}
```

## 校验

```sh
butler validate
butler tree
```

`validate` 会同时加载 `config.jsonc` 和 `plan.jsonc`，检查已配置的 MQTT/邮件参数、节点结构、调度、动作和通知渠道。当前命令在发现错误时只输出第一个错误；此外，未知 JSON 字段会被忽略，Interval 和 TriggerOffset 的部分非法写法也可能漏检，因此校验通过不代表所有运行时依赖都已连通。
