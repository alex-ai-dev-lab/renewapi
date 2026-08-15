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
