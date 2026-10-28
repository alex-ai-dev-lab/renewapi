# Claude 扩展字段协议故障切换修复

日期：2026-10-28。修复已提交，并随 v1.5.1 发布上线；部署验收见
[发布归档](2026-10-deploy-v1.5.1.md)。

## 背景

jlp 的 `/v1/messages` 请求日志包含 `tools.defer_loading`、`thinking.display`，
以及未记录具体字段名的 Claude 扩展不兼容错误；`context_management` 只是测试
fixture 示例，不能据此认定为用户实际字段。全局模型端点规则把 Grok 4.x 归一化到
Responses；这些字段没有等价的 Responses 表示。旧实现把转换失败标为
`SkipRetry`，因此每个请求只尝试一个渠道，并把最终错误公开成容量不足。

## 实施

- 为“源请求有效但目标协议无法表达”的转换错误增加类型，和 malformed 请求区分。
- 对全局归一化且目标为 Responses 的普通 Anthropic 渠道，转换失败时恢复该渠道
  原生 Messages 适配器，保留主机、密钥、模型映射、扩展字段和计费上下文。
- 显式模型端点、固定渠道、Codex profile 或要求 Codex identity 的渠道不绕过绑定。
- 协议不兼容错误允许候选切换，但不计入渠道禁用、模型健康或路由冷却；无兼容路径
  时返回固定 HTTP 400 公共提示，管理员日志保留原始诊断。
- 原生回退沿用已执行的输入参数覆盖，避免在发送阶段再次执行非幂等操作。

## 验证

`TestClaudeProtocolMismatchFailsOverWithoutLosingExtensions` 覆盖流式/非流式原生
回退、无兼容渠道、固定路由、malformed 输入、显式 Responses 绑定、同渠道回退、
Codex profile/identity 保护及单次参数覆盖。相关 `types`、`service/openaicompat`、
`service`、`controller`、`pkg/compat/errornorm`、`relay` 测试均通过。

最终在 Go 1.25.1 / Windows amd64、CGO 启用环境通过：

- `go test ./... -count=1 -timeout 300s`（包含今天的加密历史回归）。
- `go vet ./types ./service/openaicompat ./service ./controller ./pkg/compat/errornorm ./relay`。
- `go build ./...`、改动 Go 文件 `gofmt -l` 空输出、`git diff --check`。

没有调用真实上游重放 jlp 正文，也没有验证线上哪个渠道支持这些扩展；集成测试使用
本地 HTTP 服务器与内存 SQLite。发布时合成请求已在候选与公网验证：无兼容路径明确
400、公共提示不泄露字段、管理员原始诊断保留、零扣费且模型失败计数不变。
线上成功仍取决于兼容渠道的实际可用性。今天的加密历史修复一并保留；无数据库结构变化。
