# 当前任务：菜单分类与混合模型映射（2026-10-02）

> 历史阶段记录，已完成并随 v1.0.0 发布。下文的分支和禁止提交限制仅对应初期开发，后续授权和最终验证见[正式发布归档](../2026-10-release-v1.0.0.md)。

继续在 D:/Code/renewapi/ui-20261001/renewapi / feature/obsidian-ui 实施已批准计划。保留全部已存在Obsidian改动；不reset、不切分支、不commit/push/deploy、不读历史设计或记忆、不截图。上一轮docs/OBSIDIAN_UI.md为当前交付说明而非新视觉来源。本轮详细已批准计划位于 C:/Users/Ken/.claude/plans/iterative-squishing-mountain.md。

## 基线
Go1.25.1 `go test ./common ./model ./service/channelconfig ./relay/helper ./relay/common ./controller` 在本轮修改前通过。Bun1.3.14 已安装；上一轮96测试、build:check通过，全仓lint/format有已记录的历史问题，不做无关重构。

## 通用契约（主代理负责 common/model_mapping.go + tests）
保持现有 ParseModelMapping(raw) (map[string][]string,error) 和 ResolveModelMappingCandidates(map,source) ([]string,error) 的旧格式兼容接口。
新增：
```
type ModelMappingRule struct {
 ID string `json:"id"`
 From string `json:"from"`
 To string `json:"to"`
 Priority int64 `json:"priority"`
 Enabled bool `json:"enabled"`
}
type ModelMappingCandidate struct { Model string; RuleID string; Priority int64 }
type ModelMappingConfig struct {
 Version int // 1 legacy, 2 direct rules
 Legacy map[string][]string
 Rules []ModelMappingRule
}
func ParseModelMappingConfig(raw string) (*ModelMappingConfig,error)
func (c *ModelMappingConfig) IsRules() bool
func (c *ModelMappingConfig) Sources() []string // sorted unique, only enabled v2 sources
func (c *ModelMappingConfig) Candidates(source string) ([]ModelMappingCandidate,error)
func ValidateModelMapping(raw string) error // strict writes; checks all legacy expansions
func ValidateModelMappingChange(previous, next string) error // unchanged legacy retained; rejects silent v2→legacy downgrade (use v2 rules:[] to clear)
```
新版wire仍为model_mapping字段中的JSON字符串：{"version":2,"rules":[{"id":"r1","from":"alias","to":"upstream","priority":100,"enabled":true}]}。
- id/from/to required；priority absent→0，enabled absent→true；显式false不被覆盖。
- v2 direct matching original requested source once；A→B/B→A合法；priority大优先，同分原array顺序；sources/targets可重复；candidate按target稳定去重。
- Legacy支持string/string[]、递归链、depth32、自映射终点、终点稳定去重；null runtime仍按旧empty读取，但严格新写入拒绝null。
- Bounds: input1MiB、最多1024条v2 rule或legacy source、单模型名最多255个Unicode字符、id最多128字符、priority signed32-bit整数、legacy总边4096、每源最多128终点、每次解析候选展开工作8192步/整份写入验证65536步。Main如发现需调整会消息同步。
- ParseModelMapping facade对v2返回compiled direct候选map，供旧来源枚举代码兼容；**运行时必须用新Config.Candidates，不要再把v2 map传给legacy递归Resolve**。
- Envelope识别只在version值为数字时判断；旧模型名叫version/rules且值为string/string[]仍按legacy处理。v2未知字段/未知版本报错。

## Ownership（避免并行改同文件）
A 菜单代理：前端sidebar metadata/data/config/view、command-navigation、布局类型、菜单设置与profile菜单配置；后端仅 common/constants.go、model/option.go、controller/option_validation.go、controller/misc.go 中新增 SidebarTaskSectionOrder（不要改映射相关文件）。
B 映射UI代理：features/channels/components/model-mapping-editor、lib/model-mapping-validation、channel-form、editor/channel-editor、tag-batch-edit-dialog及其测试/必要types。不碰API并发协议或其它agentGo文件。
C Relay执行代理：controller/relay.go、relay/helper/model_mapped.go、relay/common/channel_failover.go/relay_info.go、service/channel_select.go、compatible/responses等wire改写与对应tests。不要改common/model_mapping.go或保存入口。
D 保存验证代理：model/channel.go（包含GetRoutingModels/EditChannelByTag）、controller/channel.go、service/channelconfig/*及对应tests。不要改parser/relay/菜单option文件。
Main：common/model_mapping.go及tests、集成QA、翻译合并、文档和最终全量验证。

## 跨代理约定
- 新增文案用t()，写自己的 tasks/active/mapping-i18n-{menu|editor}.json（顶层en/zh字典），不要直接并发修改locale。
- 遵守现有AGPL notices、JSON common wrapper、Go1.25.1/Bun conventions。不依赖未知新库。
- 旧未编辑记录不自动升级。新版无匹配时passthrough原模型；禁用规则不移除显式Models。
- UI默认空新配置用v2；旧配置显式升级，预览旧展开最终候选并保持顺序，不简单把旧链边照抄成direct。批量空字段=不更新，清空显式v2 emptyrules。
- 一对多不并发；已开始输出/观测usage或结算/取消/未知执行状态不得新增盲重发。
- 同渠道候选必须计入统一实际调用预算，每次写attempt record；不得用删除已尝试渠道集合绕过，不能重置budget。
- 上游真实请求body.model必须匹配候选，保持客户授权/计费模型不变，不使用另一个switchRelayFallbackModel路径。

## 完成标准
菜单Root/Admin/User与隐藏开关、顺序兼容；editor首次添加/回显/草稿/重置/双格式；parser/保存/缓存/批量一致；真实主relay A→B→下一渠道闭环、预算/流/取消/费用安全；Bun+Go+真实浏览器测试。不把仅helper测试当主循环集成验证。

## 实施记录
- 菜单投影、稳定身份、独立新顺序、旧模块权限及配置写回已完成，菜单专项28测试通过。
- editor一次新增及焦点、草稿保留、双格式、显式升级/清空、标签“不更新”语义已完成，真实Go+Chrome专项保存/重载流程通过。
- common双格式解析/严格写入已完成。common/json.go新增唯一JSON成员读取器；旧持久化legacy读取兼容与严格新写入区分。
- Go与TypeScript共享20条契约夹具，包含数值2.0/1e2、特殊模型名、互映、旧链、重复字段、优先级和禁用语义，双侧通过。
- 统一保存和tag原子更新已完成；独立审查发现的无If-Match预读竞态已用受控交错复现并修复为内部CAS绑定预读版本。
- relay真实主循环、wire改写、全局预算、零预扣active账务状态已完成。核心19个HTTP+SQLite场景及安全回归通过。
- 第二轮独立审查发现param override固定model造成X→X重复调用，已补非passthrough真HTTP回归并修复：独立记录最终wire模型，阻止固定/改变model及operations场景的同渠候选；legacy非model覆盖仍可候选。所有operations配置的保守限制见docs/MODEL_MAPPING.md。
- 日志UI新增实际目标及规则ID展示，兼容旧记录；新增SSR转义/字段兼容测试。
- 复制渠道旧测试曾依赖其它测试预先安装DOM且无RouterContext，已只修测试装配，独立测试通过，生产复制逻辑未改。
- 最终前端：146 tests / 0 fail / 444 assertions，typecheck 与生产构建通过。
- 最终后端：common、model、service/channelconfig、relay/helper、relay/common、relay、controller、service 八包测试通过，Go构建通过。
- 最终真实Go+SQLite+Chrome：453项检查通过，0 issues / 0 page errors / 0 console errors / 0 network failures，覆盖五档宽度、深浅模式、菜单及映射保存/重载/升级/清空。
- 147个前端改动文件与HEAD的ESLint规则/严重性对比无新增诊断；保留已记录的无关全仓lint/格式基线。
- P2最终方案保留独立UpstreamWireModel，不改adapter.UpstreamModelName；固定model（含固定为首候选）及operations关闭同渠候选，legacy非model覆盖保持可用。实际HTTP回归证明X→X重复已消除。
- 有实际模型数据时价格次文字透明度导致的对比度问题已在卡片/表格修复，最终全站复验通过。
- 本轮源码、JSON契约、ADR与操作说明均在当前工作副本，未提交/推送/部署，未截图。
