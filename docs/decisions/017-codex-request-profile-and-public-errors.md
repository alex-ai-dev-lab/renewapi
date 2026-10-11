# ADR-017：Codex 出站规范、字段保留和可解释错误

状态：已接受；2026-10-11。源码变更，未部署。

## 背景

第三方 Responses 上游对请求形状有不同限制。#257 的对照请求证明，缺少 `include` 或 `prompt_cache_key` 都会触发 `invalid codex request`，`temperature`/`top_p` 也被拒绝，但 `max_output_tokens=64` 被接受。仅转换端点、注入 Codex 安装标识不能解决这些差异；Chat DTO 往返还会丢失 include，旧转换器未复制缓存键。

## 决策

- 渠道 settings 增加 `responses_request_profile`（auto/responses/codex），与认证、URL、安装标识和端点路由独立。原生 Codex 固定采用 Codex 规范；普通 API Key 渠道需要明确选择 codex，不依据 GPT 名称或 identity 标志猜测能力。无需数据库迁移。
- 三种入口在共享传输发送前执行最终 JSON 处理，HTTP、原生 Responses WebSocket、渠道测试复用。先执行新 `upstream_param_override`，再执行禁止字段策略、Codex 规范及客户端身份注入。原有 ParamOverride 保留其既有阶段，不自动拆分或重放旧操作组；Claude → Responses 改用直连转换器。
- Codex 合并去重 `reasoning.encrypted_content`，保留已有缓存键，缺省时按用户/渠道/会话命名空间生成；无会话标识则使用请求级随机值，同请求同渠道重试稳定。缓存键不替代历史。缺省 instructions 补空字符串，规范化字符串 input/content，使用 store=false 和 SSE，非流式客户端仍收到原协议 JSON。
- 移除 Codex 不支持的 temperature/top_p 并记录路径；普通 Responses 保留。显式 store=true、conversation、无法解析的 previous_response_id 不静默改写；返回具体参数错误。原有压缩操作和压缩上下文链路保持独立。
- `responses_supports_max_output_tokens` 是三态能力。API Key 聚合渠道默认保留输出上限；原生 Codex 默认明确拒绝带硬上限的请求，管理员确认支持后可选择 true。禁止提前静默删除上限。#257 后续部署可选择 codex + true。
- DTO 保留同协议顶层扩展以及 Claude 内容块、消息、来源、thinking、Chat 消息、Responses reasoning 扩展。最终合并不能复活被删除的声明字段（包括大小写别名）。OpenAI response choice 使用具名 message 字段，避免嵌入类型的自定义 JSON 方法吞掉 choice 外层字段。
- Claude → Responses 保留工具 ID、参数、混合内容、JSON schema 和显式 false/0；工具失败以结构化 JSON 文本携带 `is_error` 和内容。缓存 TTL 没有等价实现，仅保留内容并记录能力差异；thinking budget 映射为 effort 是近似。未知执行字段、外国签名、服务端工具/状态依赖明确拒绝，不伪造密文或工具结果。同协议原生字段继续保留。
- 发送体修改后重建 Content-Length/GetBody，透明 HTTP 重试重放实际已发送的字节。管理员成功/失败详情及尝试链记录字段处理动作，不记录正文、完整缓存键或凭证；移除无条件 `lossy=false` 日志。
- 公共错误保留本地余额/额度、权限、限流、参数、模型不可用及压缩能力错误；路由失败不暴露内部渠道/分组。上游只有明确模型错误码映射为固定安全提示（model_not_found/model_unavailable）；通用 404、上游余额/凭证错误和未知错误仍隐藏详情。HTTP 与流错误事件遵循相同分类，管理员保留原诊断。

## UI 与验证

渠道 Models 列恢复历史提交 `7f7b188` 的厂商分组图标，OpenAI/Claude 固定前两位，最多五槽，支持点击和键盘弹层。修复表格可见列改变后行缓存未刷新的问题。

回归包括 3 入口 × 2 流式模式 × 2 轮工具交互、旧/目标参数覆盖各执行一次、实际出站 JSON、原生 WebSocket、渠道测试/HTTP 重放、字段保留及错误可信边界。具体命令和本地浏览器验收结果见任务记录。

## 兼容范围

此变更不会把任意供应商的所有扩展伪装成 Codex 功能。已实现的字段映射与原有协议能力校验共同决定可接受范围；未知语义需要后续增加明确转换实现。没有修改生产渠道、认证方式、价格或模型路由规则。

自动生成的 prompt_cache_key 使用 64 字符完整 SHA256，不添加可见前缀，以满足上游长度限制；命名空间已经包含在哈希输入中。客户端提供的缓存键保持原值。
