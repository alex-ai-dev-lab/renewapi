# 从源码构建

常规安装使用 [Releases 离线镜像](release-distribution.md)。需要本地开发镜像时，
在仓库根目录使用 Docker 与 Buildx 构建当前检出的源码。

## 本地加载

Linux / Bash：

```bash
NEWAPI_IMAGE=renewapi:dev bash scripts/local-build.sh --load
```

Windows / PowerShell：

```powershell
.\scripts\local-build.ps1 -Image renewapi:dev -Load
```

这两个加载命令当前构建 `linux/amd64` 镜像。正式的 amd64、arm64 归档由统一
Actions 工作流在各自架构的 runner 构建并发布到 Releases；本地脚本的推送选项已停用。

未指定版本时，脚本读取根目录 `VERSION`。构建同时记录源码提交、UTC 时间、
构建渠道和已审计上游基线，分别通过 `VERSION`、`COMMIT_SHA`、`BUILD_DATE`、
`BUILD_CHANNEL`、`UPSTREAM_REF` 构建参数写入镜像标签元数据。

## 构建内容

- `frontend-default-builder` 和 `frontend-classic-builder` 分别构建两套前端。
- `backend-builder` 将前端产物嵌入 Go 程序，运行镜像中的程序路径为 `/app/new-api`。
- `runtime` 使用 Alpine，包含 CA 证书、健康检查工具和许可证；入口脚本按
  `PUID` / `PGID` 处理数据目录后，以对应用户运行程序。

两套前端资源都是当前构建依赖，不能只因文件内容相同就删除其中一份。
开发环境另有 `Dockerfile.dev`、`docker-compose.dev.yml` 和根目录 `makefile`，
用于后端容器配合前端开发服务器，不能替代正式镜像。

## 启动与检查

本地镜像加载后，将部署目录 `.env` 的 `NEWAPI_IMAGE` 设为 `renewapi:dev`，
按[部署说明](deploy.md)初始化数据库并启动。默认 Compose 对外端口为 `3002`：

```bash
curl -fsS http://127.0.0.1:3002/api/status
```

直接启动二进制时内部默认端口为 `3000`，可用 `PORT` 调整。
