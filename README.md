<div align="center">

# RenewAPI

### A source-first AI gateway for multi-provider infrastructure

[![CI](https://github.com/alex-ai-dev-lab/renewapi/actions/workflows/build-release.yml/badge.svg?branch=main)](https://github.com/alex-ai-dev-lab/renewapi/actions/workflows/build-release.yml)
[![License](https://img.shields.io/badge/license-AGPL--3.0-2F855A.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8.svg?logo=go&logoColor=white)](go.mod)

OpenAI- and Claude-compatible APIs, provider-aware routing, billing, rate
limits, observability, and security controls in one operational surface.

[Documentation](docs/) | [Deployment](docs/deploy.md) | [Releases](https://github.com/alex-ai-dev-lab/renewapi/releases) | [Security](SECURITY.md)

</div>

RenewAPI is an independently maintained source fork of
[QuantumNous/new-api](https://github.com/QuantumNous/new-api). It preserves the
upstream API surface and data model where compatibility matters, while adding a
source-first build workflow, fork-owned runtime guardrails, and a deliberate
upstream review process documented in [`UPSTREAM_PORTS.md`](UPSTREAM_PORTS.md).

## The Shape

```text
Client SDKs / Claude CLI
          |
          v
OpenAI- and Claude-compatible API
          |
          v
Relay + routing + billing + guardrails
          |
          v
Configured provider channels
```

## Why RenewAPI

| Capability | What it means in practice |
| --- | --- |
| **One stable API** | Keep client integrations consistent while providers and models change behind the gateway. |
| **Provider-aware routing** | Select, retry, rate-limit, and observe channels without scattering provider logic across clients. |
| **Operational guardrails** | Apply compatibility hooks, anti-poison profiles, response checks, and redacted evidence logging at the relay boundary. |
| **Production ownership** | Build from the checked-out source, publish immutable image metadata, and keep release identity separate from upstream tags. |

## At A Glance

| Area | Stack |
| --- | --- |
| Backend | Go, Gin, GORM v2 |
| Frontend | React 19, TypeScript, Rsbuild, Base UI, Tailwind CSS |
| Frontend tooling | Bun |
| Classic frontend | React 18, Vite, Semi Design; see the [retirement window](docs/classic-frontend-retirement.md) |
| Data | SQLite by default; MySQL and PostgreSQL supported |
| Cache | Redis and in-process cache |
| Authentication | JWT, WebAuthn / Passkeys, OAuth and OIDC providers |
| Runtime | Multi-stage Docker build with a non-root Alpine image |

## Start In Minutes

The default Compose path uses SQLite and persists data, logs, and public assets
under the current directory.

```bash
git clone https://github.com/alex-ai-dev-lab/renewapi.git
cd renewapi
cp .env.example .env
openssl rand -hex 32   # put the result in .env as SESSION_SECRET
docker compose -f compose.yaml up -d
```

Check the service:

```bash
curl -fsS http://127.0.0.1:3002/api/status
```

The container image is published at
`ghcr.io/alex-ai-dev-lab/renewapi`. Select a different image or host port in
`.env` with `NEWAPI_IMAGE` and `NEWAPI_HOST_PORT`. See the
[deployment guide](docs/deploy.md) for MySQL, PostgreSQL, proxy, permission,
and rollback details.

## Build From Source

RenewAPI is built from the checked-out source tree. A local single-architecture
build is enough for development:

```powershell
.\scripts\local-build.ps1 `
  -Image ghcr.io/alex-ai-dev-lab/renewapi:dev `
  -Load
```

For a multi-architecture build or a release build, use the existing workflow
and release scripts. Do not treat a raw upstream `v*` tag as a RenewAPI product
release.

Upstream changes are reviewed and ported selectively with the
[`UPSTREAM_PORTS.md`](UPSTREAM_PORTS.md) ledger and the
[`scripts/check-upstream.*`](scripts/) helpers.

## Project Map

| Path | Responsibility |
| --- | --- |
| `router/`, `controller/`, `service/`, `model/` | HTTP surface, business logic, and persistence |
| `relay/` | Provider adapters and the relay pipeline |
| `middleware/`, `setting/`, `common/` | Runtime policy, configuration, and shared infrastructure |
| `web/default/` | Primary React frontend |
| `web/classic/` | Compatibility frontend retained during the documented retirement window |
| `pkg/compat/` | Fork-owned relay hooks, normalization, scheduling, and price sync |
| `relay/antipoison/` | Channel profiles, response checks, scanners, and tool-call guards |
| `scripts/`, `docs/` | Build, deploy, rollback, upstream review, and maintenance workflows |

## Compatibility And Security

- The Go module path remains `github.com/QuantumNous/new-api` to reduce
  upstream integration friction.
- SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6 remain supported through
  cross-database-safe migrations and queries.
- Anti-poison controls are channel-scoped and optional. They cover envelope
  validation, opaque payload scanning, tool-call guard checks, and redacted
  evidence logs without injecting canary text into normal user requests.
- Secrets belong in `Token/` or local `.env` files. Never commit credentials,
  private keys, or generated server access files.

Read the [architecture notes](docs/ARCHITECTURE.md),
[maintenance workflow](docs/MAINTENANCE.md), and
[security policy](SECURITY.md) before making infrastructure or provider-level
changes.

## Releases

RenewAPI product tags use the `renewapi-v<version>` namespace so imported
upstream tags remain untouched. The current product prerelease is
[`v1.0.0-rc.3`](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.0.0-rc.3).

Prerelease images use three identities:

```text
ghcr.io/alex-ai-dev-lab/renewapi:<version>
ghcr.io/alex-ai-dev-lab/renewapi:rc
ghcr.io/alex-ai-dev-lab/renewapi:sha-<source-commit>
```

The stable `latest` alias is reserved for stable product releases. See
[`docs/decisions/003-build-metadata-release-channels.md`](docs/decisions/003-build-metadata-release-channels.md)
for the release-channel contract.

## License

RenewAPI follows the upstream
[GNU Affero General Public License v3.0](LICENSE). Attribution and the
additional notices in [`NOTICE`](NOTICE) must remain intact.
