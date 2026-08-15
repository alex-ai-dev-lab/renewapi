# AGENTS.md — RenewAPI Engineering Rules

## Overview

RenewAPI is an independently maintained downstream source fork of
[QuantumNous/new-api](https://github.com/QuantumNous/new-api). It is an AI API
gateway/proxy built with Go that aggregates 40+ upstream AI providers behind a
unified API, with user management, billing, rate limiting, and an admin
dashboard.

NewAPI is the principal upstream/reference source, not a repository to merge
or rebase blindly. Sub2API, CPA, and other external projects are reference
sources only. Preserve RenewAPI compatibility, security controls, architecture,
and product identity when adapting external behavior.

## Sources of truth

- Current project state: `docs/PROJECT_STATE.md`
- Architecture: `docs/ARCHITECTURE.md`
- Maintenance workflow: `docs/MAINTENANCE.md`
- Upstream baseline: `UPSTREAM.md`
- Upstream dispositions: `UPSTREAM_PORTS.md`
- Durable decisions: `docs/decisions/`
- Active large tasks: `tasks/active/`
- Product version: `VERSION`
- User-visible changes: `CHANGELOG.md`
- Implementation history: Git

Do not rely on prior Codex conversation as an authoritative project record.

## Before substantial work

1. Inspect `git status`, the current branch, and recent relevant history.
2. Read `docs/PROJECT_STATE.md` and identify any matching task in
   `tasks/active/`.
3. Read only the architecture, ADR, upstream, and development documents that
   are relevant to the change.
4. Inspect the actual implementation and tests before proposing or editing.

Unknown state must be recorded as unknown or verified, never guessed.

## Tech Stack

- **Backend**: Go 1.22+, Gin web framework, GORM v2 ORM
- **Frontend**: React 19, TypeScript, Rsbuild, Base UI, Tailwind CSS
- **Databases**: SQLite, MySQL, PostgreSQL (all three must be supported)
- **Cache**: Redis (go-redis) + in-memory cache
- **Auth**: JWT, WebAuthn/Passkeys, OAuth (GitHub, Discord, OIDC, etc.)
- **Frontend package manager**: Bun (preferred over npm/yarn/pnpm)

## Architecture

Layered architecture: Router -> Controller -> Service -> Model

```
router/        — HTTP routing (API, relay, dashboard, web)
controller/    — Request handlers
service/       — Business logic
model/         — Data models and DB access (GORM)
relay/         — AI API relay/proxy with provider adapters
  relay/channel/ — Provider-specific adapters (openai/, claude/, gemini/, aws/, etc.)
middleware/    — Auth, rate limiting, CORS, logging, distribution
setting/       — Configuration management (ratio, model, operation, system, performance)
common/        — Shared utilities (JSON, crypto, Redis, env, rate-limit, etc.)
dto/           — Data transfer objects (request/response structs)
constant/      — Constants (API types, channel types, context keys)
types/         — Type definitions (relay formats, file sources, errors)
i18n/          — Backend internationalization (go-i18n, en/zh)
oauth/         — OAuth provider implementations
pkg/           — Internal packages (cachex, ionet)
web/             — Frontend themes container
 web/default/   — Default frontend (React 19, Rsbuild, Base UI, Tailwind)
  web/classic/   — Classic frontend (React 18, Vite, Semi Design)
  web/default/src/i18n/ — Frontend internationalization (i18next, zh/en/fr/ru/ja/vi)
```

## Internationalization (i18n)

### Backend (`i18n/`)
- Library: `nicksnyder/go-i18n/v2`
- Languages: en, zh

### Frontend (`web/default/src/i18n/`)
- Library: `i18next` + `react-i18next` + `i18next-browser-languagedetector`
- Languages: en (base), zh (fallback), fr, ru, ja, vi
- Translation files: `web/default/src/i18n/locales/{lang}.json` — flat JSON, keys are English source strings
- Usage: `useTranslation()` hook, call `t('English key')` in components
- CLI tools: `bun run i18n:sync` (from `web/default/`)

## Rules

### Rule 0: Durable maintenance records

Permanent rules belong here; current state belongs in `docs/PROJECT_STATE.md`;
long-lived reasons belong in ADRs; task handoffs belong in `tasks/active/` or
`tasks/archive/`; detailed implementation history belongs in Git. Keep these
files concise and do not turn them into a changelog.

Before finishing substantial work, run relevant formatting, lint, tests, and
build checks, then update the task, project state, ADR, upstream ledger, or
changelog when those facts changed.

### Rule 0.1: Version and release identity

`VERSION` is the pure RenewAPI product version only. RenewAPI product Git tags
use the `renewapi-` prefix (for example, `renewapi-v1.0.0-rc.1`) so raw
`v*` upstream NewAPI tags remain in the shared Git namespace untouched. Keep
product version, upstream baseline, Git commit, build time, and build channel
separate. Historical tags and releases are immutable. Main builds are `edge`
plus `sha-*`; product tags are the only path to prerelease/stable product
releases; never make a SHA build the stable `latest` alias.

### Rule 0.2: Upstream synchronization

Do not merge or rebase the unrelated NewAPI history. For each candidate change,
inspect intent, compare the RenewAPI implementation, port or adapt only the
needed behavior, test it, and record the disposition in `UPSTREAM_PORTS.md`.

### Rule 0.3: Durable decisions and task scope

Create or update an ADR for architecture, compatibility, persistence,
security, routing, release/version, or upstream-policy decisions that future
maintainers might otherwise remove. Use a task document for work spanning
multiple modules or sessions; archive it only after validation succeeds.

### Rule 1: JSON Package — Use `common/json.go`

All JSON marshal/unmarshal operations MUST use the wrapper functions in `common/json.go`:

- `common.Marshal(v any) ([]byte, error)`
- `common.Unmarshal(data []byte, v any) error`
- `common.UnmarshalJsonStr(data string, v any) error`
- `common.DecodeJson(reader io.Reader, v any) error`
- `common.GetJsonType(data json.RawMessage) string`

Do NOT directly import or call `encoding/json` in business code. These wrappers exist for consistency and future extensibility (e.g., swapping to a faster JSON library).

Note: `json.RawMessage`, `json.Number`, and other type definitions from `encoding/json` may still be referenced as types, but actual marshal/unmarshal calls must go through `common.*`.

### Rule 2: Database Compatibility — SQLite, MySQL >= 5.7.8, PostgreSQL >= 9.6

All database code MUST be fully compatible with all three databases simultaneously.

**Use GORM abstractions:**
- Prefer GORM methods (`Create`, `Find`, `Where`, `Updates`, etc.) over raw SQL.
- Let GORM handle primary key generation — do not use `AUTO_INCREMENT` or `SERIAL` directly.

**When raw SQL is unavoidable:**
- Column quoting differs: PostgreSQL uses `"column"`, MySQL/SQLite uses `` `column` ``.
- Use `commonGroupCol`, `commonKeyCol` variables from `model/main.go` for reserved-word columns like `group` and `key`.
- Boolean values differ: PostgreSQL uses `true`/`false`, MySQL/SQLite uses `1`/`0`. Use `commonTrueVal`/`commonFalseVal`.
- Use `common.UsingPostgreSQL`, `common.UsingSQLite`, `common.UsingMySQL` flags to branch DB-specific logic.

**Forbidden without cross-DB fallback:**
- MySQL-only functions (e.g., `GROUP_CONCAT` without PostgreSQL `STRING_AGG` equivalent)
- PostgreSQL-only operators (e.g., `@>`, `?`, `JSONB` operators)
- `ALTER COLUMN` in SQLite (unsupported — use column-add workaround)
- Database-specific column types without fallback — use `TEXT` instead of `JSONB` for JSON storage

**Migrations:**
- Ensure all migrations work on all three databases.
- For SQLite, use `ALTER TABLE ... ADD COLUMN` instead of `ALTER COLUMN` (see `model/main.go` for patterns).

### Rule 3: Frontend — Prefer Bun

Use `bun` as the preferred package manager and script runner for the frontend (`web/default/` directory):
- `bun install` for dependency installation
- `bun run dev` for development server
- `bun run build` for production build
- `bun run i18n:*` for i18n tooling

### Rule 4: New Channel StreamOptions Support

When implementing a new channel:
- Confirm whether the provider supports `StreamOptions`.
- If supported, add the channel to `streamSupportedChannels`.

### Rule 5: Upstream Relay Request DTOs — Preserve Explicit Zero Values

For request structs that are parsed from client JSON and then re-marshaled to upstream providers (especially relay/convert paths):

- Optional scalar fields MUST use pointer types with `omitempty` (e.g. `*int`, `*uint`, `*float64`, `*bool`), not non-pointer scalars.
- Semantics MUST be:
  - field absent in client JSON => `nil` => omitted on marshal;
  - field explicitly set to zero/false => non-`nil` pointer => must still be sent upstream.
- Avoid using non-pointer scalars with `omitempty` for optional request parameters, because zero values (`0`, `0.0`, `false`) will be silently dropped during marshal.

### Rule 6: Billing Expression System — Read `pkg/billingexpr/expr.md`

When working on tiered/dynamic billing (expression-based pricing), you MUST read `pkg/billingexpr/expr.md` first. It documents the design philosophy, expression language (variables, functions, examples), full system architecture (editor → storage → pre-consume → settlement → log display), token normalization rules (`p`/`c` auto-exclusion), quota conversion, and expression versioning. All code changes to the billing expression system must follow the patterns described in that document.
