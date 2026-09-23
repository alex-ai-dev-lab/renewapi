# 2026 年 9 月检查修复与发布整理

状态：进行中。用户要求修复主分支失败检查，清理旧 Releases 镜像及 Actions 历史，并让后续构建自动发布到 Releases。

## 基线与边界

- 基线：`61185489a653f9f96a31ce2793c133d6308ceb57`，已合并到 `main`。
- 按用户最新要求，已有工作分支合并到 `main`，后续直接在主分支修改，不再新建分支；原 `agent/06-billing` 工作树及未提交修改保持不变。
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

- 已将修复、维护和 Billing 工作分支的独有提交合入 `main`；升级分支此前已经合入。New API 的 `upstream/main` 只作参考，不合并其独立历史。原 Billing 工作树 14 项用户删除保持不变。
- 已保存失败运行日志、浏览器附件和 SHA256；88 项检查中 6 项失败均来自渠道列表 404 及其暗色错误提示对比度。
- 完整 API、Dashboard、Relay、Video 路由树下复现 GET/POST `/api/channel` 返回 404；新增显式集合路径后，GET/POST/PUT 均直接经过管理员鉴权。
- `go test ./router -count=1`、前端 typecheck、Sonner 的实际 eslint/Prettier 检查和 default 构建通过。本地 Node 驱动原 88 项浏览器检查全部通过，console/page errors 均为 0；Windows Bun 驱动 Chromium 管道超时不计为通过。
- 清理前清单：74 个 Releases、366 个附件，合计 2,674,118,239 字节；700 次 Actions 运行，均已结束。
- 本地 GitHub 凭据只有 repo/workflow 等权限，Packages API 明确返回缺少 read:packages；一次性维护工作流将使用仓库自身授权，先盘点并校验替代 Release 后再清理。
- 一次性维护分支 `maintenance/2026-09-ghcr-cleanup` 的运行 `35804583924` 已成功读取仓库所属镜像：685 个版本，其中 131 个带标签。清单摘要为 `d0a8c531399c075d38500692b3a0358790c6c35893a9a359d2625e4dd0c36d8a`。按最新要求合并时保留数据库验证工作流，将维护入口放入独立的 `registry-maintenance.yml`，仅允许主分支手动执行，完成后移除。
- 统一发布已接入可复用浏览器和四数据库工作流；去除独立触发、重复提交状态及过时自改写工作流。镜像在原生 amd64/arm64 runner 构建，离线归档只进入 Releases；草稿下载验证后公开，禁止覆盖已发布附件。
- 本地发布身份测试 4 组通过，覆盖每次构建独立身份、源码锁定、非法输入、正式/候选版本和自定义源码包。Actionlint 1.7.12、4 个 Bash 脚本及 3 个 PowerShell 脚本语法检查通过；没有运行生产部署脚本。
- 合并后的首次统一运行 `35806177406` 已通过四版本数据库验证，但 Linux runner 的 ShellCheck 对校验和生成管道报 SC2094。本地原先未安装 ShellCheck，因此此前 Actionlint 结果不包含这项检查。现改为在临时目录生成清单再复制，并用校验下载的 ShellCheck 0.11.0 联合 Actionlint 复测通过；改动文件的 ESLint 增加 `--no-ignore`，避免 UI 文件被默认忽略。
