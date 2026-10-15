# ADR-018：Responses 工具输出与首语义识别

状态：已接受；2026-10-15。本地修复，未发布部署。

## 背景

同一渠道的自定义工具请求，直连返回完整工具调用，经 v1.4.1 转发却产生空流 502。
共享首语义分类器遗漏自定义工具；后续审计还发现混合文本与工具输出被互斥处理、
SSE 聚合混用 item_id/call_id，以及 DTO 丢失自定义输入。

## 决策

- 首语义识别包含自定义工具，以及 computer/local_shell/shell/apply_patch 的具名动作字段。
  不根据任意 `_call` 后缀放行；进度、空消息、usage 与无动作工具前导仍不能触发提交。
- 有内容的 text/refusal/arguments/custom input 完成事件可触发提交，但不能替代协议终态。
  已识别错误仍优先拒绝；首语义后不跨渠道拼流的边界保持不变。
- 文本与函数调用可以共存。Responses → Chat 的流式、非流式，以及聚合回退均保留两者；
  finish_reason 按实际工具调用与现有 incomplete 规则生成。
- 聚合按 call_id 保存调用，item_id 仅用于关联输入事件；完成参数可补齐缺失的 delta。
  自定义 input 使用可空指针，区分缺省和合法空字符串，不按 JSON 函数参数处理。

## 边界与验证

流转发和 Responses/compact 聚合复用 helper.SSEScanner，多行 JSON 在事件边界合并，
event 字段可补齐 JSON 缺省 type；保留既有无空行单行上游的即时处理与 EOF 回退。
整个累计事件受缓冲上限约束，避免逐行均未超限却累计无限增长。
Responses 工具 item、arguments.done 和终态快照共用参数合并逻辑，仅发出未发送后缀；
与已发送内容冲突则报错，不拼接互相矛盾的 JSON。Chat → Messages 非流式按实际内容
独立保留文字与工具，终止原因继续独立映射。
缓存选项在 Responses/Chat 间保留；Messages 无等价项的惩罚/缓存参数、未实现完整
输出转换的 logprobs 明确拒绝，原生协议转发不受此限制。

Responses → Messages 也必须先经过共享工具声明提升和命名空间展平，并保存逆向名称映射；
验证与实际转换使用同一份准备结果，避免验证通过但发送空工具列表。
每次渠道尝试从请求的模式、路径和转换链基线开始，并重新生成当前渠道的防投毒契约。
清理发生在尝试边界，而非 InitChannelMeta，避免嵌套转换清掉当前尝试刚注入的契约。
请求级重试预算、渠道排除集合和计费上下文继续跨尝试保留。

客户端操作工具识别针对原生 SSE；不代表所有新工具均支持 Chat/Claude 转换或非流式 DTO。
跨协议安全桥已有的能力拒绝与 Anti-Poison 策略继续适用。本次没有扩展工具执行权限。
新增失败复现与回归覆盖混合输出、调用身份、完成事件回退、custom input 与空流负例。
详细证据及命令见 tasks/active/2026-10-custom-tool-stream.md。
