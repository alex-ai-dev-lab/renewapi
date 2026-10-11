# Codex 请求规范化、字段保留与渠道模型图标

状态：源码实现及本地验收完成，2026-10-11；已随 v1.4.1 提交、发布并部署，最终源码 08b2b35；实现基线 main@7c4337d。部署与异常记录见 [发布归档](2026-10-deploy-v1.4.1.md)。

## 用户授权范围

- 实现三入口共享的 Codex 请求规范化及原生/跨协议字段保留，沿用现有认证、端点、重试与计费边界。
- 查询历史模型图标实现，恢复渠道 Models 列厂商图标，优先 GPT/OpenAI 与 Claude。
- 模型不存在、模型不可用、RenewAPI 余额/令牌额度等明确错误不再统一改成 capacity 文案；未知上游故障保留公共兜底。
- 用户随后明确授权检查提交、Actions 构建到 Releases、服务器代理下载和无缝切换，成功后清理本次中间文件及备份。部署记录另见 v1.4.0 发布任务。

## 已有证据

生产 #257 对照：include 或 prompt_cache_key 缺失各自触发 invalid codex request；temperature/top_p 被明确拒绝。省略 instructions/store、字符串 content、max_output_tokens=64 有成功样本。Chat 桥接丢失 include 和 PromptCacheKey 已在本地出站集成测试复现。

详细诊断保存在工作区 audit/20261011-invalid-codex；设计以该目录 PROPOSED_FIX.md 为起点。
历史图标实现：7f7b188（厂商分组，OpenAI/Claude 固定前两位，最多五槽，溢出弹层）。

## 开发与验收

- [x] 共享规范化、独立渠道配置、目标覆盖阶段、缓存键作用域和管理员字段处理记录。
- [x] Chat/Responses/Claude 字段映射与同协议扩展保留；不静默丢失工具失败语义。
- [x] HTTP/WS/渠道测试及流式/非流式边界的出站回归。
- [x] 本地可解释错误分类和各协议公共响应回归。
- [x] 模型图标恢复、GPT/Claude 优先、展开/键盘/移动显示验证。
- [x] Go、前端类型/格式/lint/构建及浏览器相关检查；更新 ADR、状态与变更记录。

## 已完成行为与边界

- 独立 `responses_request_profile` 与 `responses_supports_max_output_tokens`，原生 Codex 自动启用，普通 API Key 渠道显式配置；不修改现有认证/URL。新 `upstream_param_override` 在转换后执行一次，旧规则不重放。
- include 合并、缓存键保留/生成、instructions/input/store/stream 规范化及采样参数处理集中在最终发送层。原生 Codex 不再静默删掉硬输出上限；不支持的存储/历史依赖明确拒绝。
- Claude 直连 Responses，保留工具失败和 JSON schema/strict；同协议原生签名、缓存 TTL、扩展字段继续透传。未知跨协议执行字段明确报错，优化提示/思考预算差异记录到管理员详情和尝试链。
- 本地可解释错误不被 errornorm 覆写；上游模型错误码仅映射安全分类，未知故障继续兜底。详情保持管理员可见。
- 恢复历史模型图标，OpenAI/Claude 优先；修正当前 Lobe 图标库没有 OpenAI.Color 导致的字母 O 回退。桌面和手机都支持图标弹层，修复切换 Models 可见列时行缓存未更新。
- HTTP Content-Length/GetBody 使用最终发送体，跨渠道重试重置强制 SSE 状态；故障切换只结算成功尝试。

## 验证结果

- `go test ./...` 通过；最后的字段别名、防丢失和故障切换修补后，相关 dto/service/controller/relay/common/channel 回归全部通过。
- `go vet ./dto ./types ./service/... ./relay/... ./controller ./middleware ./pkg/compat/errornorm` 通过；最后修改的包已复查。
- 原有三协议 36 场景两轮工具矩阵通过；新增 Codex 三入口 × 流式/非流式共 6 场景，每个两轮，核验真实 HTTP 出站参数、旧覆盖与目标 append 各执行一次。
- 原生 Claude 出站携带缓存 TTL、thinking 扩展、未知原生字段和签名；WebSocket Codex、渠道测试/HTTP 重放、Codex → 普通 Responses 故障切换以及单次成功结算通过。
- DTO 往返、扩展不能复活被移除字段（含大小写别名）、缓存键作用域、工具失败/混合内容、输出限制/状态拒绝、错误可信边界通过。
- Bun 47 项相关测试通过；前端 typecheck、所改文件 ESLint/Prettier、生产构建通过；Go 构建通过。
- `scripts/qa/codex-channel-icons.cjs` 使用最终本地构建和一次性 SQLite：1440px/390px 图标顺序、最多五槽、真实 OpenAI SVG、键盘弹层、渠道编辑保存规范/false 能力/目标覆盖均通过，浏览器 page error 为零。
- `git diff --check` 通过。完整记录与截图在工作区 `audit/20261011-invalid-codex/`（不含提交凭证）；保留预先存在的 UI 复查修改。

## 后续使用

#257 已选择 Codex 兼容，并将输出 token 上限设为支持（先前已实测 64 可用）。普通 Responses 渠道可继续 auto/responses。未知签名、服务端工具/状态等不具有通用跨协议表示，不承诺任意供应商扩展都能无损转为 Codex；详见 ADR-017。

发布阶段补充：真实上游暴露 72 字符生成缓存键过长，v1.4.1 改为 64 字符并补长度回归。#257 Chat 缺省补全请求成功，#138 Claude 工具失败历史请求成功，明确模型错误验证通过。临时测试输出和一次性二进制/数据库已按用户授权清理，保留脱敏报告和最终截图。
