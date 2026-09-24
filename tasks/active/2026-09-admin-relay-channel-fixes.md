# 管理员超时、日志和渠道连续保存修复

基线：main / 9e1d6c709。实现位于 main-integration；旧 source 工作树的补丁不作为发布输入。

- 后台系统行为增加 RelayFirstByteTimeout，整数 1–86400 秒；数据库保存值优先于环境变量，默认 15 秒。配置读取加锁，输入空格规范化。
- 外层首语义输出 watchdog 与扫描器共享请求快照，覆盖响应头等待；有有效输出后才启用 STREAMING_TIMEOUT 空闲期限。非流式和 Realtime 的既有行为不扩展。
- default/classic 管理员错误列表优先显示 admin_info.real_error；保留主分支已有的普通用户日志脱敏。历史 real_error 可以直接展示，不修改历史记录。
- 渠道保存成功采用后端返回的新配置及 config_version，同步详情缓存并重置表单基线，清空已提交密钥输入；真实跨页面并发冲突仍保留保护。
- 已通过全量 `go test ./...`、相关包 `go vet`、配置重载与空格校验、首语义/空闲期限隔离、响应头等待超时，以及前端连续保存版本递增和冲突处理测试（5 项）。
- default TypeScript/生产构建/改动文件 ESLint、classic 生产构建通过。classic 构建仍提示既有 Browserslist 数据过期及大 chunk 警告。
- 未推送、发布或部署；未进行真实浏览器及生产验收。
