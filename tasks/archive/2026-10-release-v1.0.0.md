# v1.0.0 正式发布（2026-10-02）

状态：完成。产品源码提交 `20cdd2d0c6dd83c5b2e58b852f11684223475d47`。

## 目标与授权

承接 Claude 会话 `d5ea77fd-5c94-47a9-9071-9aec52241146`。用户要求提交 GitHub 主分支，通过 Actions 构建 Releases 镜像，并清理全部旧发布、附件和标签；已在原会话明确确认清理范围。不得新建分支，不要求截图，不涉及服务器部署。

当前工作副本 `D:/Code/renewapi/ui-20261001/renewapi`，分支 `main`，基线 `d35afe9528fb630199b8b40e1a528f33afe3239d`。旧 UI/映射任务文档中的“不提交/不切分支”等阶段限制已被这次发布授权替代。

## 已接续的工作

- 保留 Obsidian UI、任务菜单、混合模型映射及全部回归测试。
- 修复剩余 React 生命周期及查询依赖 lint 错误，保留钱包加载/重试、订阅状态与用户选择。
- 版权头工具保留格式化器所需的空行，避免 CSS 在两个门禁之间来回变化。
- 浏览器门禁改为验证新版首页的实际地址、注册开关、可聚焦代码及复制结果，保留 API 故障恢复断言。此轮无截图，仍保存 ARIA、可访问性与功能检查证据。
- `VERSION` 已设为 `v1.0.0`；更新 changelog、发布政策和 ADR-013。

## 验证与结果

- 本轮前端 150 测试 / 454 断言通过；改动文件 lint/format、全前端版权头、typecheck、生产构建通过。
- 全部跟踪 Go 包测试通过；相关九包 vet、Go 构建、依赖完整性/tidy、JSON 边界、actionlint、发布身份 4 测试及暂存差异检查通过。
- 发布用浏览器门禁本地验证：108 页面场景、4 个故障恢复用例以及高级设置全流程通过，控制台/页面错误为 0。额外 Obsidian 五档宽度及混合映射全站回归 453 项通过，0 issues / 0 page errors / 0 console errors。
- `487ba7e` 已推送 main；首次 Actions `36980576787` 的四个数据库版本全部通过，前端测试因旧测试的全局 Sonner mock 污染失败。已在本地按 CI 顺序复现，改为单方法 spy 并逐项恢复；两文件专项 6 测试通过，不改产品逻辑。
- 修复提交 `20cdd2d` 的[主分支构建](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/36981012896)和[正式标签构建](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/36982359440)全部成功。两套前端、Go 完整测试/竞态检查/vet/build、四版本数据库、两组浏览器门禁及原生双架构镜像构建均通过。
- [RenewAPI v1.0.0](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.0.0) 已公开，非草稿、非预发行，已设为 latest，标签 `renewapi-v1.0.0`。共 13 个附件，镜像 `renewapi:1.0.0`。
- Actions 上传后重新下载全部附件并校验；本地再次下载校验全部 12 个被列入 CHECKSUMS 的附件，并读取两个 Docker 归档内的 manifest/config，确认真实架构、镜像标签和源码 SHA 均一致。
- 正式版成功后删除 5 个历史 Release 和本轮 1 个临时源码构建版，共 78 个附件；原有 79 个标签加临时构建标签共 80 个，通过带预期引用值的原子 Git push 删除。API 复查仅剩 1 个正式 Release 和 1 个正式标签。
- 未删除分支、main 提交历史、Actions 记录或生产数据。未部署服务器，未截图。
- 原始清单和本地执行日志在 `.cache/release-tools/`；永久清理记录见 [JSON 清单](2026-10-release-v1.0.0-cleanup.json)。最后仅文档归档提交使用 `[skip ci]`，避免重新生成待清理的源码预发行包；产品源码及已验证镜像不变。

## 镜像 SHA256

| 文件 | SHA256 |
| --- | --- |
| `renewapi-1.0.0-linux-amd64.tar.gz` | `27be571d5ad769d5b1cbf1257c5c27ec5502e92ee0bbe1e838e8a8a72be3b046` |
| `renewapi-1.0.0-linux-arm64.tar.gz` | `9b26b7569d6a40396cafcf58c6b04712a56609a1c97fbed1fdbfa37b68f48381` |
