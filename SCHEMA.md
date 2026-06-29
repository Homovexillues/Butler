# plan.jsonc 字段速查（写计划文件 / AI 编辑前必读）

> butler 的计划真相源。用 JSONC（支持注释、尾逗号，由 hujson 解析）。
> 字段名**大小写不敏感**（`title` / `Title` 均可），下表用首字母大写形式。
> ⚠️ 本文件须与 `internal/parser/plan_dto.go` 的 `PlanNode` 同步——改字段时一并更新。

## 顶层结构

```jsonc
{
  "Children": [ /* PlanNode 数组 */ ]
}
```

## 节点（PlanNode）字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `Title` | string | 节点标题（必填，会作为通知标题） |
| `Body` | string | 通知正文 |
| `Once` | string | 一次性调度，格式 `"2006-01-02 15:04:05"`（带年） |
| `Solar` | string | 公历每年，格式 `"01-02 15:04:05"`（无年，每年重复） |
| `Lunar` | string | 农历每年，格式 `"01-02 15:04:05"`（农历月-日，无年） |
| `Cron` | string | 标准 5 段 cron：`分 时 日 月 周`，如 `"0 9 * * *"` |
| `NotifyOffset` | []string | 提前提醒偏移，见下 |
| `Channels` | []string | 通知渠道，见下。可被子节点继承 |
| `Children` | []PlanNode | 子节点，构成树 |

## 两类节点（互斥）

每个节点**要么是分组、要么是叶子**，不能既是又是：

- **分组节点**：有 `Children`，**无任何调度字段**。只用于归类 + 向下传递 `Channels`。
- **叶子节点**：有**恰好一个**调度字段（Once/Solar/Lunar/Cron 四选一），**无 Children**。

违反（同时有调度和 Children、或一个都没有、或多个调度字段）会被 `butler validate` 报错。

## 调度字段格式

| 字段 | 格式 | 示例 | 含义 |
|---|---|---|---|
| `Once` | `YYYY-MM-DD HH:mm:ss` | `"2026-06-30 14:00:00"` | 仅一次，过期不再触发 |
| `Solar` | `MM-DD HH:mm:ss` | `"03-05 09:00:00"` | 每年公历 3 月 5 日 9 点 |
| `Lunar` | `MM-DD HH:mm:ss` | `"05-08 09:00:00"` | 每年农历五月初八 9 点 |
| `Cron` | `m h dom mon dow` | `"0 9 * * 1"` | 每周一 9 点 |

## Channels（通知渠道）

可用渠道：`system`（桌面弹窗）、`mqtt`（推送到 MQTT，安卓端订阅）、`email`（邮件兜底）、`messagebox`（模态弹窗）。

- 节点未设 `Channels` 时**继承父节点**的（逐层向下）。
- 叶子节点最终必须有至少一个渠道（自身或继承），否则通知发不出，`validate` 会报错。
- 渠道名拼错（不在上述集合）会被 `validate` 报错。

## NotifyOffset（提前提醒偏移）

让一个目标派生出多次"提前"提醒。格式 `"T-<n>d"`（提前 n 天）或 `"T-<n>h"`（提前 n 小时）。**偏移必须带单位 `d`/`h`**——无单位（如 `"T-0"`、`"T"`）一律非法报错。"目标当天/当时"用 `"T-0d"` 表示。

```jsonc
"NotifyOffset": ["T-3d", "T-0d"]   // 目标前 3 天、目标当天 各提醒一次
```

- **不设 NotifyOffset**：在目标时刻当天/当时触发一次（默认行为）。
- **设了 NotifyOffset**：只在 `目标 + 各偏移` 触发；**此时不再自动包含"目标当天"**——要当天提醒须显式写 `"T-0d"`。

## 完整示例

```jsonc
{
  "Children": [
    {
      "Title": "家人生日",
      "Channels": ["email", "mqtt"],   // 下面的孩子都继承这俩渠道
      "Children": [
        { "Title": "娘的生日", "Body": "记得打电话", "Lunar": "01-29 07:00:00" },
        { "Title": "爹的生日", "Body": "记得打电话", "Lunar": "10-18 07:00:00" }
      ]
    },
    {
      "Title": "日常作息",
      "Channels": ["system", "mqtt"],
      "Children": [
        { "Title": "吃饭", "Body": "到点吃饭", "Cron": "50 11 * * *" },
        { "Title": "下班打卡", "Body": "记得打卡", "Cron": "10 18 * * *" }
      ]
    },
    {
      "Title": "交季度报告",
      "Body": "deadline",
      "Once": "2026-06-30 14:00:00",
      "Channels": ["mqtt"],
      "NotifyOffset": ["T-3d", "T-1d"]
    }
  ]
}
```

## 校验

写完用 `butler validate` 检查（结构、调度格式、渠道名、偏移格式都会查）。
