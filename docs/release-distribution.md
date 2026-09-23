# Releases 镜像分发

RenewAPI 仅在 [GitHub Releases](https://github.com/alex-ai-dev-lab/renewapi/releases)
发布构建产物。主分支、产品标签和手动构建都必须先通过统一检查；不再向 GHCR 发布。

## 下载与校验

每次成功发布包含 amd64、arm64 两个 Docker 离线归档、`CHECKSUMS.txt`、
`build-info.json`、镜像元数据、Compose 文件、`default.env.example` 和 QA 证据。
按服务器架构下载对应归档及配置，保存在同一目录，然后执行：

```bash
sha256sum --ignore-missing -c CHECKSUMS.txt
docker load -i "$IMAGE_ARCHIVE"
```

`IMAGE_ARCHIVE` 应设置为实际下载的 `renewapi-…-linux-amd64.tar.gz` 或
`renewapi-…-linux-arm64.tar.gz` 文件名。校验必须成功后再导入镜像。

首次安装可复制 `default.env.example` 为 `.env` 并设置随机 `SESSION_SECRET`。
该文件已经固定当前 Release 的 `NEWAPI_IMAGE`，随后使用随附 Compose 文件。
已有安装保留原 `.env`、挂载目录及数据库，只更新明确需要的配置和镜像标签。
Compose 的 `pull_policy: never` 确保使用导入的本地镜像；没有导入时直接失败，
不会尝试从远端仓库拉取。

## 发布与证据

统一流程包含 Go、双前端、MySQL 5.7/8.4、PostgreSQL 9.6/16、页面和设置
实际浏览器检查。检查失败时不发布镜像。草稿附件重新下载并通过 SHA256
校验后才公开；同一运行重试不覆盖已经公开的文件。

源码构建使用 `renewapi-build-<sha>-<run>` 预发行标签，产品版本仍由 `VERSION`
和 `renewapi-v<version>` 标签管理。源码构建不会冒充正式版本。

旧 GHCR 推送脚本已停用。本地构建使用 `scripts/local-build.*` 的加载模式；
部署脚本要求服务器已经导入镜像，不再执行镜像仓库拉取。迁移和生产切换
需要按实际安装的备份、迁移和回滚流程单独执行，构建发布不会自动部署服务器。
