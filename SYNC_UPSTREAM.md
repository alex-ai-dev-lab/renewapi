# Sync Upstream

This repository and `QuantumNous/new-api` have unrelated Git histories. The supported strategy is selective manual porting recorded in `UPSTREAM_PORTS.md`; direct merge or rebase is intentionally refused.

```bash
bash scripts/check-upstream.sh
bash scripts/sync-upstream.sh --port
```

PowerShell:

```powershell
.\scripts\check-upstream.ps1
.\scripts\sync-upstream.ps1 -Mode port
```

The check script fetches upstream, reads `Audited-Upstream-Ref` from the ledger, and lists only commits still awaiting review. Port updates in small behavior-focused commits, preserve fork-specific compatibility/security behavior, and add focused tests.

If a future repository rewrite establishes a genuine common ancestor, `--merge`/`--rebase` (or `-Mode merge`/`-Mode rebase`) become available again. Use dry-run first and run at least:

```bash
go test ./relay/antipoison ./service ./model ./controller
```

Rebuild both frontends before publishing a Docker image.
