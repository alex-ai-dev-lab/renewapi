# v1.1.1 Logo 原色修复与部署

状态：完成。2026-10-03。

- 用户报告网站 Logo 变黑；线上确认图片正常加载，但 SnowAPI 首页与侧栏使用 brightness(0) 把 JPG 全部不透明像素压黑；深色模式压白，公共页面另外存在反色。
- 移除首页、侧栏、认证、公共页面和升级展示的 Logo 变色滤镜，保留配置图片原色，不修改功能图标或业务规则。
- 本地使用线上同一张 Logo，首页/登录/文档/已登录侧栏 × 明暗 × 桌面/手机共 16 个场景通过，计算样式 filter=none；类型、lint、格式、版权及生产构建通过。
- 正式 [v1.1.1](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.1.1)，源码 `db8a6949d0111785344bd3311eea61fa5932ecdd`；[正式 Actions](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/37102098155) 和主分支构建 37102098105 运行同一源码。正式版完整前后端、四数据库、108 页面、8 项权限、4 项恢复及高级设置检查均通过。
- 服务器通过既有代理下载并核验 Release 的 SHA256、架构和源码标签；镜像 `renewapi:1.1.1` / `sha256:e83b289cbc76a9fd71a19180e3bd73b096edaca303a88d63a0161033c942af2d`。
- 在线 SQLite 备份、隔离迁移及启动通过，14 核心表和结构指纹一致。原环境、会话密钥、数据/日志挂载和 BILLING_LEDGER_MODE=off 保留。
- 候选 `new-api-v1-1-1` 在 3015 预热，Caddy 2.11.6 于 2026-10-03T14:23:59.918443+08:00 热切换，仅更改本服务的两个 upstream。既有 5 分钟流连接保护保留。
- 旧容器 `new-api-v1-1-0` 连接归零后停止，退出码 0，未超时。规范 Compose 和本地健康地址同步至新容器。
- 本轮每 5 秒采样，公网 28 次连续探测，0 失败；最大采样耗时 575.52 ms。
- 上线后首页/登录/文档 × 明暗 × 桌面/手机共 12 项通过，Logo 加载正常、无变色滤镜且无页面/控制台错误。后台侧栏按本地真实登录回归验证；生产未修改用户凭据。
- 最终状态 healthy，SQLite quick_check=ok，核心计数：{"users": 18, "tokens": 61, "channels": 104, "options": 86, "user_sessions": 1}。
- 保留旧容器、镜像、数据库备份和前两轮静态资源缓存；本轮缓存 1622 个资源于 `/var/lib/caddy/renewapi-legacy-assets/v1.1.1`。
- 服务端证据与回退脚本：`/home/ken/services/new-api/backups/deploy-v1.1.1-20261003T062232Z`；`rollback.py` 预热旧版后恢复入口，不覆盖当前数据库。
- 本地证据：忽略目录 `.cache/logo-fix/`；文档归档不改变已发布源码。
