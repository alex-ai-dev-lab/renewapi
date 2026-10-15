# ADR-019：Caddy 请求体期限与长流连接

状态：已接受，2026-10-15；生产配置已热加载，应用仍为 v1.4.2。

## 证据

用户请求 `202610150932255892363098268d9d6zBEAsfUV` 已收到 2403 个有效 Responses 事件并转发 2399 个，59.902 秒后下游取消，内部为 499/client_gone；不是空语义输出。公共脱敏文案 model_capacity 不能作为上游实际满载的证据。

同渠道长流对照：容器本机直连 78.24 秒收到完成事件；Caddy 本机 TLS 入口 60.02 秒截断；公网入口 60.05 秒截断。调整后公网 Responses 长流 81.54 秒收到 response.completed，Claude 长流 74.05 秒收到 message_stop，两者均有成功计费日志。

Caddy 2.11.6 的 [app.go](https://github.com/caddyserver/caddy/blob/v2.11.6/modules/caddyhttp/app.go) 将未设置的 ReadIdleTimeout 默认补为一分钟；[IdleTimeoutReader.Read](https://github.com/caddyserver/caddy/blob/v2.11.6/modules/caddyhttp/idletimeout.go) 在每次读取请求体前设置底层读取期限，EOF 后没有清除。请求体传输已结束，期限仍可能在响应期间触发连接取消。默认值没有出现在 admin API 的原始 JSON 中，不能据字段缺失推断无超时。

## 决策

在 RenewAPI 的监听器范围配置 `timeouts { read_body_idle -1s }`，避免该期限截断长响应；`0` 会重新采用一分钟默认值，不能用于关闭。保留其他监听器行为、响应写入空闲保护、请求头上限、反代立即刷新和重载连接保护。

这是入站服务器的请求体期限，反代 `transport http` 的 read_timeout/write_timeout 设置为 0 无法替代。使用 `servers :1443` 精确限制生产变更范围；其他安装应替换为实际监听地址。不能通过放宽客户端超时、跨渠道重试已提交的流、伪造完成事件或关闭终态校验来修复。

后续 Caddy 升级只有在确认 EOF 后期限被正确清理，且超过一分钟的真实长流通过后，才可移除此兼容配置。源码链接固定于本次实际审阅的版本。

## 验收要求

每次反代或超时设置变更，除短请求和状态探测外，必须通过至少一次超过 60 秒且收到协议完成事件的真实请求；覆盖常用客户端协议。记录持续时间、终态和请求关联信息，不保存用户正文或凭证。失败不能仅以 HTTP 200 认定成功。

应用的未知上游错误仍按既有规则脱敏；管理员应依据 admin_info.real_error、状态和尝试链诊断。真实余额不足、并发限制及首语义超时分别处理，不将它们归因于本期限。
