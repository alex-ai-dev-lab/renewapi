# Claude 扩展字段协议故障切换修复

日期：2026-10-28。范围：本地源码，未提交、未发布、未部署。

## 背景

jlp 的 `/v1/messages` 请求携带 `tools.defer_loading`、`thinking.display` 和
`context_management` 等 Claude 字段。全局模型端点规则把 Grok 4.x 归一化到
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
本地 HTTP 服务器与内存 SQLite。线上是否能成功仍取决于兼容渠道的实际可用性。
现有加密历史修复与其他未提交改动保留；本次无配置或数据库结构迁移。
