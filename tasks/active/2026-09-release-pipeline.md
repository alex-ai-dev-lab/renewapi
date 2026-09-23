# 2026 年 9 月检查修复与发布整理

状态：进行中。用户要求修复主分支失败检查，清理旧 Releases 镜像及 Actions 历史，并让后续构建自动发布到 Releases。

## 基线与边界

- 基线：`61185489a653f9f96a31ce2793c133d6308ceb57`，已合并到 `main`。
- 开发分支：`fix/2026-09-release-pipeline`；原 `agent/06-billing` 工作树及未提交修改保持不变。
- 失败运行：`35802758510`，失败步骤为 Secondary Surfaces 实际浏览器检查。
- 先保存失败证据及旧发布清单，修复、验证新的自动发布，再清理旧 Releases 与已结束运行。
- 不删除 Git tags、不修改生产数据库、不部署生产；用户已明确要求清理 GHCR 历史镜像，后续仅通过 Releases 分发。
- 用户本次明确授权历史 Release 清理和自动 Release 发布，覆盖此前维护文档中的对应限制；产品版本与稳定镜像别名仍按既有规则区分。

## 待办

- 定位并修复真实浏览器失败，不能跳过或放宽有效检查。
- 统一自动发布流水线，检查通过后将镜像、校验和及 QA 证据写入 Release。
- 验证真实 Actions 运行和 Release 下载产物。
- 按已确认清单清理旧 Releases 和 Actions 历史，保存本地清理记录。
- 更新维护规则、发布 ADR 和项目状态。

## 当前证据

- 已保存失败运行日志、浏览器附件和 SHA256；88 项检查中 6 项失败均来自渠道列表 404 及其暗色错误提示对比度。
- 完整 API、Dashboard、Relay、Video 路由树下复现 GET/POST `/api/channel` 返回 404；新增显式集合路径后，GET/POST/PUT 均直接经过管理员鉴权。
- `go test ./router -count=1`、前端 typecheck、Sonner 的实际 eslint/Prettier 检查和 default 构建通过。实际浏览器复测继续执行。
- 清理前清单：74 个 Releases、366 个附件，合计 2,674,118,239 字节；700 次 Actions 运行，均已结束。
- 本地 GitHub 凭据只有 repo/workflow 等权限，Packages API 明确返回缺少 read:packages；一次性维护工作流将使用仓库自身授权，先盘点并校验替代 Release 后再清理。
