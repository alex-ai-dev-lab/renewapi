# NewAPI / Sub2API / CPA 更新检查

状态：检查完成，2026-10-17。交付候选清单与最小适配方案；后续修复尚未实施，不代表候选问题已关闭。

用户要求：最后检查三个参考源是否有适合导入的更新或升级。工作副本 `D:/Code/renewapi/repair-20261017`，main，HEAD `32f0d2bf9a4e41b1564dce167d5a434c9129eaa3`，保留上一轮未提交修复。未提交、推送、部署或导入业务代码，没有启动子代理。

固定参考 HEAD：

- QuantumNous/new-api：`3e2ce2c8856e32ca7054d673f8cd059ae466e80c`。
- Wei-Shaw/sub2api：`b0a02a3304a0a78cdd003ddda1dd23ba02c03084`。
- router-for-me/CLIProxyAPI：`911f49cbd49f4fe2516979754a7488a3fd14fe64`。

列举 10-08 以来 291 条非合并提交，保存 49 个候选 patch，针对关键生产路径定向审阅；没有声明全部提交逐行审计，原全量审计基线不推进。Release API 403 后通过 GitHub latest 网页重定向核实 rc.44 / v0.2.17 / v8.0.32。

形成 U01–U13 待适配行为：账号隔离、Claude 缓存/零值、分组限流、冷却竞态、混合流、图片转换、工具失败、多行 beta、容器哨兵、Responses reasoning、Vertex、Gemini schema、chunk 恢复。对已有 thinking 扩展、SSE、TLS 超时和文本工具能力去重；不引入外部整套架构。具体证据、边界和方案见[报告](../../docs/audits/2026-10-upstream-update-review.md)，所有未实现候选在 [UPSTREAM_PORTS](../../UPSTREAM_PORTS.md) 标 DEFERRED。

依赖检查：官方支持版本为 Go1.27.2 / 1.26.9。实际 Go1.25.1、linux/amd64、CGO_ENABLED=0 的 govulncheck v1.9.0 扫描有 86 个去重告警，其中 43 个符号可达；不等于生产可利用性确认。优先升级 Go、websocket≥1.5.3、x/net≥0.60.0 与配套 x/*、x/image≥0.45.0，统一验证而非分散重复升级。首次 go run 扫描自动切换工具链，已改用独立 scanner 并固定实际工具链完成复核；初次 Linux 扫描因 Windows C 编译器不适用失败，按 Docker 的 CGO_ENABLED=0 重跑成功。

完成验证：

- Go/Bun 23 条观测、15 条观测断言；独立 overlay SQLite 冷却探针。记录当前缺口，不冒充修复后回归。
- 五个相关包回归和 service HTTP 握手/混合响应定向回归通过，共六个包。
- 17 个报告提交链接对照参考 Git 对象核实；49 patch 归档；git diff --check 通过。
- 上轮 110 文件哈希中 109 个不变，另一个为本轮主动更新的 docs/PROJECT_STATE.md。go.mod、go.sum、Docker 和 CI 未改；未重写原修复结果。
- 外部证据及 SHA256 manifest：`D:/Code/renewapi/audit/upstream-review-20261017/`，验收汇总 `review-validation.json`；详细告警与调用路径 `vulnerability-summary.json`。

后续实施按报告四组边界逐批适配并重新验收。现有旧版本回归不代替升级后的全仓、前端、三库、HTTP/SSE/WS 验收；真实 Stripe 渠道验收、旧 off 历史余额边界继续沿上轮记录。
