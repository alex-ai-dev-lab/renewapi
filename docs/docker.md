# Docker

The Dockerfile builds directly from the current source checkout.

Stages:

- `frontend-builder`: builds `web/default` and `web/classic`.
- `backend-builder`: downloads Go modules with BuildKit cache and builds `/app/new-api`.
- `runtime`: Alpine runtime with CA certificates, curl healthcheck, `newapi` user, and no source tree.

Build args:

- `VERSION`
- `COMMIT_SHA`
- `BUILD_DATE`
- `BUILD_CHANNEL`
- `UPSTREAM_REF`

`VERSION` is the RenewAPI product version. The other arguments identify the
immutable build and audited upstream state; they are recorded as OCI labels.
Product release Docker tags remove the leading `v` from `VERSION` (for example,
`VERSION=v1.0.0-rc.1` publishes `1.0.0-rc.1`).

Local build:

```powershell
.\scripts\local-build.ps1 -Image ghcr.io/alex-ai-dev-lab/renewapi:dev -Load
```

Multi-arch push:

```bash
VERSION=v1.0.0 ./scripts/local-build.sh --push
```

When `VERSION` is omitted, the local build scripts read the repository
`VERSION` file. Set `BUILD_CHANNEL` to label a local build explicitly.

Healthcheck:

```bash
curl -fsS http://127.0.0.1:3002/healthz
```
