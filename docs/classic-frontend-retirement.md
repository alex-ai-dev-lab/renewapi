# Classic frontend retirement

Status: removed from the current working tree following explicit user authorization on 2026-10-05. This does not change an already deployed server.

- `v1.1.3` is the last published image that embeds classic; its source remains available in Git history.
- `web/default` is the only frontend built by Docker, local builds and CI. Classic's React 18 / Vite / Semi Design sources, assets, manifest and lockfile are removed together.
- Existing `theme.frontend=classic` options resolve to the default frontend in memory. No database migration or destructive settings rewrite is required.
- Known classic `/console/*`, `/login` and `/register` URLs redirect to supported routes with their query strings intact. Existing password reset and OAuth routes retain their paths.
- The retired selector is removed from the settings UI. Switching `theme.frontend` is no longer a rollback mechanism; use the previous release image if a deployment needs rollback.
- Runtime caches on production servers that serve old browser chunks are outside this source cleanup.

Validation and any remaining limitations are recorded in [the current audit task](../tasks/active/2026-10-ui-accessibility-audit.md).
