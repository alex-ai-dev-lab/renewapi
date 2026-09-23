# Releases 镜像分发

RenewAPI 仅在 [GitHub Releases](https://github.com/alex-ai-dev-lab/renewapi/releases)
发布构建产物。主分支、产品标签和手动构建都先通过统一检查，不再向 GHCR 发布。
完整的首次安装命令见 [README](../README.md#快速部署)。

## 下载与校验

每次发布包含：

- `renewapi-<镜像标签>-linux-amd64.tar.gz`、`renewapi-<镜像标签>-linux-arm64.tar.gz`。
- `CHECKSUMS.txt`、`build-info.json` 和两个架构的 `image-linux-<架构>.json`。
- `compose.yaml`、`docker-compose.yml`、`default.env.example`。
- `RELEASE_NOTES.md`、页面检查、设置检查和数据库验证的三个 QA 证据包。

在同一目录保存选定架构的镜像、校验和及配置文件，全部取自同一个 Release。
`IMAGE_ARCHIVE` 设置为实际下载的镜像文件名，校验成功后再导入：

```bash
sha256sum --ignore-missing -c CHECKSUMS.txt && docker load -i "$IMAGE_ARCHIVE"
```

`--ignore-missing` 允许只下载一个架构，仍会校验目录内已下载的配置和镜像。
`build-info.json` 记录源码 SHA、本地镜像标签、产品版本和 Actions 运行编号；
环境示例已固定该 Release 的 `NEWAPI_IMAGE`。Compose 使用 `pull_policy: never`，
未导入镜像时会直接失败。

首次安装设置随机 `SESSION_SECRET`，先执行数据库迁移，再启动服务。
已有安装保留原 `.env`、挂载目录及数据库，按[部署说明](deploy.md)完成备份、
迁移和切换。发布工作流不会部署服务器。

## 检查与发布入口

[build-release.yml](../.github/workflows/build-release.yml) 是唯一发布入口。
它运行 Go 测试、竞态检查、vet/build、默认前端测试与类型检查、双前端构建、
MySQL 5.7/8.4、PostgreSQL 9.6/16 和两组实际浏览器检查。
全部通过后，在两个原生架构 runner 构建镜像并验证镜像身份及数据库迁移。
附件先上传草稿，重新下载并通过 SHA256 校验后才公开。

| 触发方式 | 发布结果 |
| --- | --- |
| 推送 `main` | `renewapi-build-<SHA 前 12 位>-<运行编号>` 源码预发行包。 |
| 推送 `renewapi-vX.Y.Z-rc.N` | 与 `VERSION` 一致的产品预发行版。 |
| 推送 `renewapi-vX.Y.Z` | 与 `VERSION` 一致的正式版，可成为最新正式发布。 |
| 手动运行 | 默认使用源码构建身份；自定义 Release 标签须以 `renewapi-build-` 或 `renewapi-source-` 开头。 |

上游原始 `v*` 标签不触发产品发布。源码构建不会成为最新正式版，应从 Releases
列表选择，不能依赖 `/releases/latest` 获取最近一次源码构建。
手动运行可用 `expected_sha` 校验源码、`base_sha` 指定完整改动范围；
`image_tag` 不能使用 `edge`、`rc`、`latest` 等保留名称。

产品版本、源码 SHA、构建时间和构建渠道分别记录。已发布附件不可覆盖，
既有 Git 标签不可移动或删除；重跑已公开的 Release 只验证原附件。
历史产物清理须有用户明确授权，具体规则见 [ADR-010](decisions/010-releases-only-distribution.md)。
本地构建使用 [local-build 脚本](docker.md)的加载模式；部署助手只使用服务器已导入的镜像。
