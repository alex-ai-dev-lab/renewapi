# RenewAPI

RenewAPI 是基于 [QuantumNous/new-api](https://github.com/QuantumNous/new-api)
独立维护的 AI API 网关，将多个模型服务接入统一接口，并提供管理控制台。

## 主要能力

- 兼容 OpenAI、Claude 等接口，支持 Chat Completions、Responses 和流式请求。
- 按渠道优先级、权重与模型能力路由，支持失败切换和流式响应保护。
- 提供用户与令牌管理、限流、用量统计、额度及订阅计费。
- 在控制台管理渠道、测试提示词、模型定价和请求日志。
- 支持渠道级 Anti-Poison 防护、能力探测与 Responses WebSocket。

## 快速部署

以下用于 **Linux 首次安装**，需要 Docker Compose v2、`curl`、`openssl` 和
`sha256sum`。镜像只通过 [GitHub Releases](https://github.com/alex-ai-dev-lab/renewapi/releases)
分发，提供 amd64、arm64 归档，不再发布到 GHCR。源码构建在列表中标为预发行版。

先选定一个 Release，将下方两个占位值替换为其实际标签和镜像附件名。
`x86_64` 主机选 `linux-amd64.tar.gz`，`aarch64` 主机选 `linux-arm64.tar.gz`；
所有文件必须来自同一个 Release。

```bash
set -eu
mkdir renewapi
cd renewapi
RELEASE_TAG='替换为所选 Release 的标签'
IMAGE_ARCHIVE='替换为该 Release 中对应架构的镜像归档文件名'
BASE_URL="https://github.com/alex-ai-dev-lab/renewapi/releases/download/$RELEASE_TAG"

for file in "$IMAGE_ARCHIVE" CHECKSUMS.txt compose.yaml default.env.example; do
  curl -fLO "$BASE_URL/$file"
done
sha256sum --ignore-missing -c CHECKSUMS.txt
docker load -i "$IMAGE_ARCHIVE"

cp default.env.example .env
SESSION_SECRET="$(openssl rand -hex 32)"
printf '\nSESSION_SECRET=%s\n' "$SESSION_SECRET" >> .env
chmod 600 .env
docker compose -f compose.yaml run --rm --no-deps new-api /app/new-api migrate --up
docker compose -f compose.yaml run --rm --no-deps new-api /app/new-api migrate --check
docker compose -f compose.yaml up -d
```

访问 `http://服务器地址:3002`，按页面完成管理员初始化，再添加渠道和访问令牌。
程序启动时只检查数据库状态，因此首次安装也需先运行迁移并确认完成。
已有安装请按[部署与升级说明](docs/deploy.md)保留原配置和数据，先备份再迁移。

## 配置与文档

| 配置 | 说明 |
| --- | --- |
| `NEWAPI_IMAGE` | Release 的环境示例已固定对应镜像标签，需先用 `docker load` 导入。 |
| `SESSION_SECRET` | 上述命令生成的随机会话密钥；重启和升级时保留。 |
| `NEWAPI_HOST_PORT` | 对外端口，默认 `3002`。 |
| `SQL_DSN` | 留空使用 SQLite；也支持 MySQL 和 PostgreSQL。 |

默认在部署目录的 `data/`、`logs/`、`public/` 保存数据库、日志及公共文件。
更多参数见[环境示例](.env.example)；生产环境应配置 HTTPS 并限制管理入口访问。

- [部署与升级](docs/deploy.md) · [Releases 下载与发布规则](docs/release-distribution.md)
- [从源码构建](docs/docker.md) · [Responses WebSocket](docs/responses-websocket.md)
- [当前项目状态](docs/PROJECT_STATE.md) · [架构](docs/ARCHITECTURE.md) · [维护流程](docs/MAINTENANCE.md)
- [安全说明](SECURITY.md) · [更新记录](CHANGELOG.md)

## 许可证与致谢

采用 [AGPL-3.0](LICENSE)。感谢 [New API](https://github.com/QuantumNous/new-api)
及其贡献者；分发时须保留 [NOTICE](NOTICE) 中的署名与附加条款，以及
[第三方许可证](THIRD-PARTY-LICENSES.md)。
