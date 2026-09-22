# Upstream Baseline

- Repository: `QuantumNous/new-api`
- Last fully audited ref: `58d4e9bd3bb035df8ea235dd682ccc8a45d0332a`
- Release boundary: `v1.0.0-rc.24` plus post-release `main`
- Audit completed: `2026-08-14`
- RenewAPI commit at audit completion: `91d636fba97864e54a9aec2f55667e14bcd6ae34`
- Audit ledger: `UPSTREAM_PORTS.md`
- Sync strategy: selective manual ports; merge/rebase is refused because no common ancestor exists.

The current checkout contains later RenewAPI commits after the audited fork
review base. They do not change the audited upstream ref; review new upstream
commits with the scripts and advance this baseline only after a complete audit.

## 2026-09-23 可靠性专项引用

- New API：`QuantumNous/new-api main@996adffe5165bd5e311e33a03a86b8aede1fe376`。
- Sub2API：`Wei-Shaw/sub2api main@20a94fbb567b62208751292ed7786b24a7e7c0fe`。
- 范围：Responses 用量、跨协议 usage、限流最终结果、错误隔离、返回模型、Options、选择器 autofocus，以及 Responses WebSocket。
- 本地行为移植和复核：`ada0a55bb`、`7c2cf87cd`、`a2321c2b1`；之前已完成的可靠性改动直接复用。

逐项 IMPLEMENT / NOOP / REJECT 及对应本地提交见 `UPSTREAM_PORTS.md`。这是限定范围的专项复审，不扩展上方完整历史审计基线，也不代表已审计全部新增功能。

## Fork Scope

The main fork-owned surfaces are compatibility bridges, security controls, billing hardening, deployment tooling, and Interface Zero frontend metadata. High-conflict areas include:

- `relay/channel/openai/*`
- `relay/helper/*`
- `service/*compat*` and quota settlement
- `controller/*compat*` and authentication
- `model/*` migrations and locking
- `web/default/src/features/system-settings/*`
- `scripts/` and `.github/workflows/`

Use the check scripts to enumerate upstream commits after the audited ref. Do not use `git diff $(git merge-base ...)` unless the script reports `shared-history`.
