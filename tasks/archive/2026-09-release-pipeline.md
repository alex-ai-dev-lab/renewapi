# 2026 年 9 月检查修复与发布整理

状态：DONE。主分支失败检查已修复，旧 Releases、Actions 和 GHCR 历史镜像已清理，后续成功构建仅发布到 Releases。

## 基线与边界

- 基线：`61185489a653f9f96a31ce2793c133d6308ceb57`，已合并到 `main`。
- 按用户最新要求，已有工作分支合并到 `main`，后续直接在主分支修改，不再新建分支；原 `agent/06-billing` 工作树及未提交修改保持不变。
- 失败运行：`35802758510`，失败步骤为 Secondary Surfaces 实际浏览器检查。
- 先保存失败证据及旧发布清单，修复、验证新的自动发布，再清理旧 Releases 与已结束运行。
- 不删除 Git tags、不修改生产数据库、不部署生产；用户已明确要求清理 GHCR 历史镜像，后续仅通过 Releases 分发。
- 用户本次明确授权历史 Release 清理和自动 Release 发布，覆盖此前维护文档中的对应限制；产品版本与稳定镜像别名仍按既有规则区分。

## 完成结果

- 已修复真实浏览器失败，并保留完整检查要求。
- 已统一发布流水线，检查通过后将双架构离线镜像、校验和及 QA 证据写入 Release。
- 已验证真实 Actions 运行、Release 下载内容、归档架构和源码身份。
- 已按冻结清单清理旧 Releases、Actions 及 GHCR，保存本地执行记录并重新查询远端。
- 已更新主分支维护规则、发布 ADR、部署说明和当前项目状态。

## 当前证据

- 已将修复、维护和 Billing 工作分支的独有提交合入 `main`；升级分支此前已经合入。New API 的 `upstream/main` 只作参考，不合并其独立历史。原 Billing 工作树 14 项用户删除保持不变。
- 已保存失败运行日志、浏览器附件和 SHA256；88 项检查中 6 项失败均来自渠道列表 404 及其暗色错误提示对比度。
- 完整 API、Dashboard、Relay、Video 路由树下复现 GET/POST `/api/channel` 返回 404；新增显式集合路径后，GET/POST/PUT 均直接经过管理员鉴权。
- `go test ./router -count=1`、前端 typecheck、Sonner 的实际 eslint/Prettier 检查和 default 构建通过。本地 Node 驱动原 88 项浏览器检查全部通过，console/page errors 均为 0；Windows Bun 驱动 Chromium 管道超时不计为通过。
- 清理前清单：74 个 Releases、366 个附件，合计 2,674,118,239 字节；700 次 Actions 运行，均已结束。
- 本地 GitHub 凭据只有 repo/workflow 等权限，Packages API 明确返回缺少 read:packages；本次通过仓库自身授权的一次性维护工作流完成盘点、替代 Release 校验和清理。
- 一次性维护分支 `maintenance/2026-09-ghcr-cleanup` 的运行 `35804583924` 曾成功读取仓库所属镜像：685 个版本，其中 131 个带标签。清单摘要为 `d0a8c531399c075d38500692b3a0358790c6c35893a9a359d2625e4dd0c36d8a`。按最新要求合并时保留数据库验证工作流，将维护入口放入独立的 `registry-maintenance.yml`；该入口已在完成清理、保存证据后移除。
- 统一发布已接入可复用浏览器和四数据库工作流；去除独立触发、重复提交状态及过时自改写工作流。镜像在原生 amd64/arm64 runner 构建，离线归档只进入 Releases；草稿下载验证后公开，禁止覆盖已发布附件。
- 本地发布身份测试 4 组通过，覆盖每次构建独立身份、源码锁定、非法输入、正式/候选版本和自定义源码包。Actionlint 1.7.12、4 个 Bash 脚本及 3 个 PowerShell 脚本语法检查通过；没有运行生产部署脚本。
- 合并后的首次统一运行 `35806177406` 已通过四版本数据库验证，但 Linux runner 的 ShellCheck 对校验和生成管道报 SC2094。本地原先未安装 ShellCheck，因此此前 Actionlint 结果不包含这项检查。现改为在临时目录生成清单再复制，并用校验下载的 ShellCheck 0.11.0 联合 Actionlint 复测通过；改动文件的 ESLint 增加 `--no-ignore`，避免 UI 文件被默认忽略。
- 主分支 `8d8bef09a9b7980c0e490a049621583bc1842724` 的统一 [Actions 35806368451](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/35806368451) 全部成功，包含完整后端、专项 race、双前端、四版本数据库、两组浏览器检查和两架构镜像构建及容器迁移。
- [首个 Releases 离线版本](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-build-8d8bef09a9b7-35806368451) 已公开。13 个附件共 87,540,374 字节，全部重新下载并核验 SHA256；直接读取两个 Docker 归档，确认架构、镜像标签和源码提交一致。
- GHCR 清理运行 `35807786536` 成功：删除前清单摘要与已核对的 685 个版本一致，删除后同一授权访问镜像包返回 HTTP 404。执行证据附件摘要为 `ec51c7660535d4517ff7923ecfe73d584ae9bc4edc8934c8233560ddadf027db`。
- 74 个旧 Releases 及 366 个附件已全部删除，重新查询只剩本次新 Release；历史 Git tags 无删除、无重指向，仅新增本次源码构建 tag。
- 702 条旧 Actions 运行已全部删除，包含此前 700 条、一次性盘点和首次统一流程的失败记录；没有失败、跳过或清单外删除。重新查询并对照原清单，旧运行剩余 0，保留本轮有效发布与清理证据。
- 27 个已无源码文件的旧工作流入口随历史运行清理消失，逐项查询均返回 HTTP 404。主分支保留统一发布入口及三个检查子工作流，一次性 GHCR 维护文件已移除。
- 持久证据目录：`D:\Code\renewapi\maintenance-backups\2026-09-23-release-cleanup`。已保存旧发布/运行/镜像元数据、失败日志、Git tags 快照、清理流水、GHCR 操作附件及完整新 Release；没有声称备份全部旧镜像二进制。

## 提交与边界

- `347398928`：渠道集合路由和暗色错误提示修复。
- `74662cce7`：统一检查、Releases 分发、离线部署配置和文档。
- `71f4bf717`、`c8a53be30`：合并已有维护和 Billing 工作分支。
- `8d8bef09a`：修复 ShellCheck 检出的校验和管道，并确保改动 UI 文件实际参与 lint。
- 后续归档提交只收尾维护入口及文档，不改变已验证的运行时代码；每次主分支提交仍执行完整发布流程。
- 原可靠性升级中的功能、NOOP 和已知协议边界见 [升级验收](2026-09-reliability-upgrade.md)。本次没有重复实现它们。
- 两套前端的存量全量 lint/format 技术债仍按原升级记录保留；改动文件检查、测试和构建通过，不将其写成全仓 lint 通过。
- 未部署生产、修改生产数据库或切换默认 Billing 模式。Git 历史、产品版本及用户原有工作树修改保持不变。
