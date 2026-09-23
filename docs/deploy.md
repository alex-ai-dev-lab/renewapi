# 部署

镜像仅通过 [GitHub Releases](https://github.com/alex-ai-dev-lab/renewapi/releases)
分发。先按 [Releases 分发说明](release-distribution.md) 下载、校验并在服务器
运行 `docker load`，再使用该 Release 对应的 `NEWAPI_IMAGE`。

`SQL_DSN` 为空时使用 SQLite，Compose 保留 `/data`、`/app/logs`、`/app/public`
三个挂载目录。已有安装保留原配置和数据，在切换前完成与当前数据库对应的
备份及迁移验证。

## 首次安装与数据库初始化

完整下载和配置命令见 [README](../README.md#快速部署)。在保存了 `.env` 与
`compose.yaml` 的部署目录中，先初始化数据库，再启动服务：

```bash
docker compose -f compose.yaml run --rm --no-deps new-api /app/new-api migrate --up
docker compose -f compose.yaml run --rm --no-deps new-api /app/new-api migrate --check
docker compose -f compose.yaml up -d
docker compose -f compose.yaml ps
```

启动过程只检查迁移是否完成，不会自动建表或升级旧结构。镜像入口需要完整
程序路径 `/app/new-api`，不能将 `migrate` 直接作为容器启动命令。
默认访问 `http://服务器地址:3002`，按页面完成管理员初始化。

`.env` 中的变量通过 Compose 的 `environment` 映射传入容器；使用额外环境
参数时，需同步检查 `compose.yaml` 中是否已声明该参数。
`docker-compose.yml` 是单服务兼容入口，仍随 Release 提供；选择一个文件后
始终通过 `-f` 使用同一份配置。

## 已有安装升级

1. 记录当前镜像、Compose 文件、配置与挂载路径，备份数据库及必要文件。
2. 在服务器下载并校验新 Release，导入镜像；保留原 `.env`，只更新所需的
   `NEWAPI_IMAGE` 和明确新增的配置，不覆盖会话密钥或数据目录。
3. 先在隔离的数据库副本验证迁移，再按部署维护窗口或已有切换流程对实际
   数据库执行上方 `migrate --up`、`migrate --check`。
4. 使用同一份 Compose 配置启动新镜像，检查容器健康、页面和实际 API。

数据库发生迁移后的回滚必须同时考虑数据库兼容性，不能只替换镜像标签。

## 部署助手

如果使用现有 Windows 部署助手，服务器必须已经导入镜像：

```powershell
.\scripts\deploy-server.ps1 -Image $ReleaseImage -Mode local -DryRun
.\scripts\deploy-server.ps1 -Image $ReleaseImage -Mode local
```

`$ReleaseImage` 取自 Release 的 `build-info.json` 或随附环境示例。助手不再
执行镜像拉取；旧 `pull` 模式会明确失败。`tar` 是历史加载模式的兼容名称，
同样要求事先导入镜像，不会上传或下载归档。

已有回滚助手继续使用服务器保留的旧本地镜像。不要在新版本验证和回滚窗口
结束前清除服务器本地镜像、数据库备份或数据卷。发布工作流不会运行部署助手。
