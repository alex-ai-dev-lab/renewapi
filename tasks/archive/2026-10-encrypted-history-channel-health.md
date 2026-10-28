# 加密会话历史提示与渠道失败计数

日期：2026-10-28。修复已提交，并按后续授权随 v1.5.1 发布上线；部署验收见
[发布归档](2026-10-deploy-v1.5.1.md)。

## 问题及结果

渠道 #267 同一 reasoning 条目重复返回 item_id 与 encrypted content 不匹配，统一失败计数在 `100-599` 配置下禁用了渠道。原有兼容重试路径虽排除该错误，统一健康路径没有排除；最终公共提示还可能变为 model_capacity。

- 将原有识别逻辑共享到 types，识别结构化错误码及原有明确校验失败文案。
- HTTP 和流错误使用固定安全提示 `The encrypted conversation history could not be verified. Please start a new conversation and try again.`，错误码 `invalid_encrypted_content`，HTTP/事件状态 400。已经提交的 SSE 连接保持原 HTTP 状态，错误事件承载分类。
- 错误规范化保留上述分类；管理员尝试链继续保存原错误。
- 统一自动禁用、渠道模型失败计数、兼容 5xx 计数和渠道冷却全部排除，包含被流解析器包装为 502 的错误。已有失败计数不因本次错误增加或重置。
- 保留现有加密历史亲和及兼容故障切换，成功时正常交付；本次不修改重试预算、账户、渠道配置和数据库。

## 验证

- `go test ./types ./service ./controller ./pkg/compat/errornorm ./relay/channel/openai -count=1 -timeout 300s` 通过。
- 对应五个包的 `go vet` 通过。
- 新增真实 relay / 内存 SQLite 回归：HTTP 400 和 SSE 错误两类上游，分别覆盖最终失败与备用成功；每组连续四次，超过默认禁用门槛。核对客户端新会话提示、渠道启用状态、统一计数为零、无渠道模型失败记录、管理员原始诊断仍在。
- 单元回归覆盖全部状态计数策略、500/502 包装、公共事件嵌套错误、无关错误不误分类、规范化双阶段和原错误不被修改。
- `go build ./...`、改动 Go 文件的 gofmt 检查及 `git diff --check` 通过。

无数据库结构或前端改动，不需要新增迁移。上述修复回归使用本地合成错误；未重放
用户加密历史。发布门禁及线上通用 API 验收见发布归档。
