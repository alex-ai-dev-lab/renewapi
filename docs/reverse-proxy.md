# 长流反向代理

Caddy 2.11.6 代理 RenewAPI 时，应显式处理入站请求体读取期限，避免已上传完成的请求在响应期间被一分钟期限取消。详见 [ADR-019](decisions/019-caddy-long-stream-deadline.md)。以下全局配置应合并进已有全局块，并将监听地址替换为实际入口：

```caddyfile
{
    servers :1443 {
        max_header_size 1MiB
        timeouts {
            read_body_idle -1s
        }
    }
}
```

该指令与版本相关，应用前使用服务器实际二进制执行 `caddy adapt` 和 `caddy validate`；不要向不支持它的旧版直接复制。数值 0 表示采用默认值，负值才会跳过该期限包装。保留响应写入空闲保护；不需要全局关闭其他站点的超时。

反代应立即刷新流，按部署需要保留重载时的连接保护，例如：

```caddyfile
reverse_proxy 127.0.0.1:3023 {
    flush_interval -1
    stream_close_delay 5m
    transport http {
        read_timeout 0
        write_timeout 0
    }
}
```

上游传输期限与入站请求体期限是不同层次，以上 transport 设置不能替代 read_body_idle。变更后对照容器直连、本机 TLS 和公网入口，验证超过 60 秒的真实 SSE 请求收到 Responses 的 response.completed 或 Messages 的 message_stop；仅检查状态接口、HTTP 200 或短工具调用不足以验收长流。
