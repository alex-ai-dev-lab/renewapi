# PR #9 补足建议：支持“单渠道任意错误连续 N 次自动禁用”

## 背景

PR #9 已经把“连续失败自动禁用”的策略做成了可配置项，这是一个很好的方向。它比直接硬编码 `400` 或直接改成所有错误都禁用更稳，因为可以通过后台开关控制不同风险等级的错误是否计入自动禁用。

不过，如果目标是支持下面这个明确需求：

> 单个渠道不管什么模型、不管什么错误类型，只要连续失败达到阈值，比如 3 次，就按照系统自动禁用流程禁用该渠道。

当前 PR 还差两个关键能力：

1. 失败计数作用域目前仍是 `channel + model`，不是纯 `channel`。
2. 错误是否计入失败仍主要依赖 `ShouldDisableChannel(err)`，即使 PR 加了一些例外配置，也不是“任意 relay 错误都计入”。

因此建议在 PR #9 的基础上补两个配置项：

1. `channel_failure_scope`
2. `count_all_relay_errors_for_disable`

这样可以同时保留默认安全行为，也能让后台明确开启“单渠道任意错误 N 次自动禁用”。

## 建议目标

默认行为保持兼容：

- 默认仍按 `channel + model` 维度计数。
- 默认仍只对系统认为应该禁用的错误计数。
- 现有用户升级后不改变已有自动禁用语义。

新增高级策略：

- 后台可切换为按 `channel` 维度计数。
- 后台可开启“所有 relay 错误都计入连续失败”。
- 开启后，`400 encrypted content could not be verified`、skip-retry client error、模型兼容错误等，只要最终落到该渠道的失败，都能计入连续失败。
- 仍然尊重全局自动禁用开关和渠道自身 `auto_ban` 开关。

## 建议新增配置

在 monitor setting 中新增：

```json
{
  "channel_failure_scope": "channel_model",
  "count_all_relay_errors_for_disable": false
}
```

字段含义：

| 配置项 | 类型 | 默认值 | 可选值 | 说明 |
| --- | --- | --- | --- | --- |
| `channel_failure_scope` | string | `channel_model` | `channel_model` / `channel` | 控制连续失败计数维度。`channel_model` 表示渠道+模型分别计数，`channel` 表示整个渠道统一计数。 |
| `count_all_relay_errors_for_disable` | bool | `false` | `true` / `false` | 是否将所有 relay 错误都计入自动禁用连续失败。默认 false，保持现有安全策略。 |

## 后端建议改法

### 1. 新增统一判断函数

建议不要让 `controller/relay.go` 直接散落判断逻辑，而是在 `service/channel.go` 中新增一个统一入口：

```go
func ShouldCountChannelFailureForDisable(err *types.NewAPIError) bool {
    if !common.AutomaticDisableChannelEnabled || err == nil {
        return false
    }

    monitorSetting := operation_setting.GetMonitorSetting()
    if monitorSetting.CountAllRelayErrorsForDisable {
        return true
    }

    return ShouldDisableChannel(err)
}
```

这样默认仍然走现有 `ShouldDisableChannel(err)`，只有后台打开 `CountAllRelayErrorsForDisable` 时才进入“任意错误都计数”模式。

注意：这个函数只决定“错误是否计数”，不直接决定是否禁用。禁用仍应走现有连续失败计数和阈值逻辑，避免绕过 PR #9 已经加好的 failure window、threshold 等配置。

### 2. 修改 relay 错误处理入口

`controller/relay.go` 里的 `processChannelError` 当前如果仍然使用：

```go
if service.ShouldDisableChannel(err) {
    service.RecordChannelFailure(...)
}
```

建议改成：

```go
if service.ShouldCountChannelFailureForDisable(err) {
    service.RecordChannelFailure(...)
}
```

这样“默认安全策略”和“任意错误计数策略”都从同一个入口生效。

### 3. 支持按 channel 维度计数

PR #9 目前类似仍会用：

```go
func channelConsecutiveFailureKey(channelID int, model string) string {
    return strconv.Itoa(channelID) + "|" + model
}
```

建议改成根据配置决定 key：

```go
func channelConsecutiveFailureKey(channelID int, model string) string {
    monitorSetting := operation_setting.GetMonitorSetting()
    if monitorSetting.ChannelFailureScope == "channel" {
        return strconv.Itoa(channelID)
    }

    return strconv.Itoa(channelID) + "|" + model
}
```

为了避免 typo，可以定义常量：

```go
const (
    ChannelFailureScopeChannelModel = "channel_model"
    ChannelFailureScopeChannel      = "channel"
)
```

默认值必须是 `channel_model`，这样不会影响现有部署。

### 4. 成功请求时的清理逻辑要一起考虑

如果 `channel_failure_scope = channel`，一次成功请求应该清掉该渠道整体连续失败计数。

如果仍是 `channel_model`，则只清理当前 `channel + model` 的失败计数。

也就是说，`RecordChannelSuccess` / `ClearChannelFailure` 一类函数不能只改失败计数 key，成功清理也必须使用同一套 key 规则，否则会出现：

- 失败按 channel 计数；
- 成功只按 channel+model 清理；
- 最后导致渠道失败次数无法被正确归零。

建议所有失败记录、成功清理、窗口判断都统一调用同一个 `channelConsecutiveFailureKey(channelID, model)`。

### 5. 是否影响定时渠道测试

如果 `controller/channel_test_scheduler.go` 也使用 `ShouldDisableChannel(err)` 或独立的禁用判断，建议一起评估。

建议原则：

- relay 在线请求失败计数使用 `ShouldCountChannelFailureForDisable`。
- 定时测试是否使用同一策略，可以单独配置或保持现有逻辑。

如果不想扩大 PR 范围，建议至少在代码注释或 PR 描述中明确：该配置只影响 relay 请求链路，不影响定时测试任务。

## 前端后台建议

在监控设置页增加两个配置项。

### 失败计数作用域

字段名：

```text
channel_failure_scope
```

建议 UI：

- Select / Segmented Control

选项：

- `渠道 + 模型`：默认，兼容当前行为。
- `渠道`：同一个渠道下所有模型共享连续失败计数。

说明文案建议：

```text
控制连续失败自动禁用的计数维度。选择“渠道”后，同一渠道下不同模型的失败会合并计数。
```

### 所有 relay 错误都计入自动禁用

字段名：

```text
count_all_relay_errors_for_disable
```

建议 UI：

- Switch

说明文案建议：

```text
开启后，任何 relay 错误都会计入连续失败自动禁用，包括 400、模型兼容错误、skip-retry client error 等。该选项更激进，建议与较短失败窗口和合适阈值配合使用。
```

## 推荐后台配置组合

如果目标就是“单渠道不管什么错误，三次就自动禁用”，后台应该这样配：

```json
{
  "automatic_disable_channel_enabled": true,
  "channel_consecutive_failure_threshold": 3,
  "channel_failure_scope": "channel",
  "count_all_relay_errors_for_disable": true
}
```

同时每个渠道自身仍需要：

```json
{
  "auto_ban": true
}
```

否则应继续尊重渠道级别 `auto_ban = false`，不自动禁用。

## 测试建议

建议至少补以下测试，避免这类策略后续回归。

### 1. 默认行为不变

条件：

- `channel_failure_scope = channel_model`
- `count_all_relay_errors_for_disable = false`

预期：

- 仍按 `channel + model` 分别计数。
- 非 `ShouldDisableChannel(err)` 命中的错误不计入禁用。

### 2. channel scope 会跨模型累计

条件：

- 阈值 3。
- `channel_failure_scope = channel`
- 同一渠道上：
  - model A 失败 1 次；
  - model B 失败 1 次；
  - model C 失败 1 次。

预期：

- 第 3 次后渠道被自动禁用。

### 3. channel_model scope 不会跨模型累计

条件：

- 阈值 3。
- `channel_failure_scope = channel_model`
- 同一渠道上：
  - model A 失败 1 次；
  - model B 失败 1 次；
  - model C 失败 1 次。

预期：

- 渠道不会被自动禁用。

### 4. 默认不计入 skip-retry 400

条件：

- `count_all_relay_errors_for_disable = false`
- 错误是 `400 encrypted content could not be verified` 或类似 skip-retry client error。

预期：

- 不计入连续失败。
- 行为和当前版本一致。

### 5. count_all 开启后 400 会计入

条件：

- `count_all_relay_errors_for_disable = true`
- 阈值 3。
- 同一渠道连续出现 3 次 `400 encrypted content could not be verified`。

预期：

- 失败计数达到 3。
- 渠道自动禁用。

### 6. 成功请求会按相同作用域清理计数

条件：

- `channel_failure_scope = channel`
- 同一渠道先失败 2 次。
- 随后任意模型成功 1 次。
- 再失败 1 次。

预期：

- 成功后该渠道整体失败计数清零。
- 最后一次失败后计数为 1，不应直接禁用。

### 7. 仍然尊重 auto_ban=false

条件：

- 全局自动禁用开启。
- `count_all_relay_errors_for_disable = true`
- 阈值 3。
- 渠道自身 `auto_ban = false`。

预期：

- 失败可以记录日志，但不应自动禁用该渠道。

## 风险说明

`count_all_relay_errors_for_disable = true` 是一个更激进的策略，可能把以下错误也计入渠道失败：

- 用户请求格式错误。
- 上游模型不支持某些参数。
- 临时内容策略错误。
- 某些非渠道可用性问题导致的 400。

因此不建议把它作为默认行为。

更稳妥的方式是：

- 默认保持 false。
- 后台显式开启。
- 配合较短的 failure window。
- 在 UI 上明确提示这是激进策略。

## 建议 PR 描述补充

可以在 PR 描述中增加：

````md
### Additional policy controls

This PR keeps the existing safe default behavior, but adds two optional controls:

- `channel_failure_scope`
  - `channel_model` by default, preserving current per channel+model counting.
  - `channel` for users who want one consecutive-failure counter per channel.
- `count_all_relay_errors_for_disable`
  - `false` by default, preserving `ShouldDisableChannel(err)` semantics.
  - `true` to count every relay error toward consecutive-failure auto-disable.

With:

```json
{
  "channel_failure_scope": "channel",
  "count_all_relay_errors_for_disable": true,
  "channel_consecutive_failure_threshold": 3
}
```

the system supports the policy: “disable a channel after any 3 consecutive relay errors on that channel,” while still respecting the global auto-disable switch and per-channel `auto_ban`.
````

## 总结

PR #9 当前方向是对的：把连续失败自动禁用做成后台可配置，比硬编码某些状态码更合理。

建议补齐的核心是：

1. 增加失败计数作用域配置：`channel_model` / `channel`。
2. 增加“所有 relay 错误都计入”的开关。
3. relay 错误处理统一通过 `ShouldCountChannelFailureForDisable(err)`。
4. 失败记录和成功清理必须使用同一套 key 规则。
5. 默认值保持现有行为，激进策略只在后台显式开启后生效。
