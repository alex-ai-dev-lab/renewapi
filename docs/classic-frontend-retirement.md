# Classic frontend retirement

Status: retired in v1.2.0 following explicit user authorization on 2026-10-05. The production deployment was upgraded and verified on the same date.

- `v1.1.3` is the last published image that embeds classic; its source remains available in Git history.
- `web/default` is the only frontend built by Docker, local builds and CI. Classic's React 18 / Vite / Semi Design sources, assets, manifest and lockfile are removed together.
- Existing `theme.frontend=classic` options resolve to the default frontend in memory. No database migration or destructive settings rewrite is required.
- Known classic `/console/*`, `/login` and `/register` URLs redirect to supported routes with their query strings intact. Existing password reset and OAuth routes retain their paths.
- The retired selector is removed from the settings UI. Switching `theme.frontend` is no longer a rollback mechanism; use the previous release image if a deployment needs rollback.
- Runtime caches on production servers that serve old browser chunks are outside this source cleanup.

Validation and scope are recorded in [the audit archive](../tasks/archive/2026-10-ui-accessibility-audit.md) and [deployment record](../tasks/archive/2026-10-deploy-v1.2.0.md).
