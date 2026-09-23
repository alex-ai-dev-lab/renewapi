# 部署

镜像仅通过 [GitHub Releases](https://github.com/alex-ai-dev-lab/renewapi/releases)
分发。先按 [Releases 分发说明](release-distribution.md) 下载、校验并在服务器
运行 `docker load`，再使用该 Release 对应的 `NEWAPI_IMAGE`。

`SQL_DSN` 为空时使用 SQLite，Compose 保留 `/data`、`/app/logs`、`/app/public`
三个挂载目录。已有安装保留原配置和数据，在切换前完成与当前数据库对应的
备份及迁移验证。

如果使用现有 Windows 部署助手，服务器必须已经导入镜像：

```powershell
.\scripts\deploy-server.ps1 -Image $ReleaseImage -Mode local -DryRun
.\scripts\deploy-server.ps1 -Image $ReleaseImage -Mode local
```

`$ReleaseImage` 取自 Release 的 `build-info.json` 或随附环境示例。助手不再
执行镜像拉取；旧 `pull` 模式会明确失败。`tar` 是历史加载模式的兼容名称，
同样要求事先导入镜像，不会上传或下载归档。

已有回滚助手继续使用服务器保留的旧本地镜像。不要在新版本验证和回滚窗口
结束前清除服务器本地镜像、数据库备份或数据卷。发布工作流不会运行部署助手，
本次仓库历史清理也不操作服务器容器或生产数据库。
