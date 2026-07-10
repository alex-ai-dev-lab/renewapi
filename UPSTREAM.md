# Upstream

- Repository: `QuantumNous/new-api`
- Last fully audited ref: `4e570389dd433a717373ce9c9b822b59f5ed3d5d`
- Tag at audit time: `v1.0.0-rc.20`
- Audit ledger: `UPSTREAM_PORTS.md`
- Sync strategy: selective manual ports; merge/rebase is refused because no common ancestor exists.

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
