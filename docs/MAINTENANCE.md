# RenewAPI Maintenance Workflow

This document is the operational entry point for future Codex sessions. The
repository is the durable memory; prior conversation is not authoritative.

## Start a substantial task

1. Run `git status`, inspect the branch and recent relevant history, and keep
   unknown user changes intact.
2. Read `AGENTS.md` and `docs/PROJECT_STATE.md`.
3. Open the matching file under `tasks/active/` when one exists.
4. Read only the relevant architecture, ADR, upstream, and development docs.
5. Inspect implementation and tests before editing.

Use a task document for work spanning modules, sessions, database migrations,
provider/routing/security changes, large upstream audits, or releases. Promote
long-lived architecture, compatibility, security, routing, upstream, and
version decisions to `docs/decisions/`.

## Upstream review

`UPSTREAM.md` records the audited NewAPI baseline. `UPSTREAM_PORTS.md` records
the disposition of each reviewed behavior. The fork has no common Git
ancestor with NewAPI, so use `scripts/check-upstream.ps1` or
`scripts/check-upstream.sh`; never merge or rebase the unrelated history.

For every candidate change: identify intent, inspect the RenewAPI equivalent,
classify it, adapt only the required behavior, test it, and record the local
reference and reason.

在仓库根目录运行以下任一命令，拉取上游并列出审计基线之后的待审提交：

```bash
bash scripts/check-upstream.sh
```

```powershell
.\scripts\check-upstream.ps1
```

基线取自 `UPSTREAM_PORTS.md` 的 `Audited-Upstream-Ref`。
现有 `sync-upstream.sh --port` / `sync-upstream.ps1 -Mode port` 仍保留为手动
审计入口；它们不会自动移植代码。当前规则禁止直接合并或变基上游历史。
选取行为后小步移植、补充验证并登记台账；发布前构建两套前端。

## Version and release policy

- `VERSION` contains only the pure RenewAPI product version, such as
  `v1.0.0-rc.1`.
- RenewAPI product Git tags use `renewapi-v<version>`, such as
  `renewapi-v1.0.0-rc.1`. Raw upstream `v*` tags never trigger product
  releases.
- `main` 和手动构建全部通过统一检查后自动发布到 GitHub Releases；不再向
  GHCR 登录或推送。源码构建使用 `renewapi-build-<sha>-<run>` 预发行标签。
- `renewapi-vX.Y.Z-rc.N` 标签保持产品预发行语义；只有正式产品标签
  `renewapi-vX.Y.Z` 可以成为最新正式 Release。
- 两个架构的 Docker 离线镜像、镜像元数据、配置、QA 证据和 SHA256 校验和
  一并上传草稿；重新下载校验成功后才公开。已公开附件不因重跑而覆盖。
- Git tags 不得重写或删除。历史 Releases、GHCR 镜像和 Actions 记录的删除
  需要用户明确授权；此次用户已授权清理，清单与执行结果单独保存。
- 具体下载和导入方式见 [Releases 分发](release-distribution.md) 与 ADR-010。
- Product-tag validation requires
  `stripPrefix(gitTag, "renewapi-") == VERSION`, source checkout integrity,
  relevant tests, build checks, and migration checks. See
  `scripts/validate-release.*` and `.github/workflows/build-release.yml`.

Before creating a product tag, validate the prepared identity with
`scripts/validate-release.ps1 -Prepare -Tag renewapi-vX.Y.Z` or
`scripts/validate-release.sh --prepare renewapi-vX.Y.Z`. The default validator
mode remains the post-tag gate and additionally requires the tag to resolve to
the checked-out clean HEAD.

产品版本以 `VERSION` 和当前发布记录为准，不在维护流程中保存会过期的“最新发布”常量。2026-09-23 当前源码声明 `v1.0.0-rc.4`；主分支检查和源码包发布分别记录证据，产品版本发布与生产部署仍需对应的独立操作。

## Finish and hand off

Run relevant format, lint, typecheck, tests, build, migration, Docker, and
workflow-static checks. Update the task, `PROJECT_STATE.md`, ADRs, upstream
records, and `CHANGELOG.md` only when their facts changed. Archive a completed
task only after validation passes; leave a concise handoff for blocked work.
