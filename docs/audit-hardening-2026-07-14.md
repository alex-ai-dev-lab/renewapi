# Runtime Hardening Audit 2026-07-14

## Reproducible Baseline

- Repository: `alex-ai-dev-lab/renewapi`
- Branch: `feature/responses-compaction-07112`
- Source baseline: `3aba7a7535478229d173b753e95106a71c34330f`
- Implementation: direct source edits; no patch or patch-application script

The resulting source is identified by the commit containing this document.

## Scope

- redact Responses request bodies and panic responses;
- validate adaptor response and usage types instead of panicking;
- keep SSE output indexes and item identities stable for tool-first streams;
- buffer tool arguments until tool identity is known;
- bound direct, HTTP proxy, SOCKS5, protected-fetch, and WebSocket connection phases;
- close duplicate HTTP transports created by concurrent cache misses;
- add baseline-aware CI for backend, both frontends, and Docker.

Existing Responses compaction routing, raw JSON preservation, model fallback,
official price synchronization, and admin UI changes remain based on the source
baseline above and are not replaced by older audit snapshots.

## Local Validation

- changed Go files: `gofmt` and `git diff --check` passed;
- `go test -race -count=1 ./middleware ./relay/responsebridge ./service` passed;
- `go test -count=1 ./...` passed;
- `go vet ./...` passed;
- `go build ./...` passed;
- default frontend: install, 12 tests, typecheck, and build passed;
- classic frontend: install and build passed;
- workflow and Compose YAML parsing passed.

Local Docker execution was unavailable because the Windows host did not have a
Docker CLI. The GitHub Actions Docker and multi-architecture image jobs are the
required container-build verification.
