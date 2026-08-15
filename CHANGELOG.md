# Changelog

## Unreleased

## v1.0.0-rc.2 - 2026-08-15

### Changed

- RequestGuard observe workers now start and stop with the server process,
  resize with configuration, and drain through a bounded shutdown path.

### Fixed

- Built-in OAuth and WeChat account binding no longer risks overwriting
  concurrent quota, status, group, or profile updates.
- Editing an existing redemption code in either frontend preserves its exact
  stored quota unless the amount or native quota is intentionally changed.
- Request replay now has regression coverage for concurrent readers,
  multipart bodies, and large disk-backed payloads.
- Async task refunds, concurrent recharge callbacks, and fallback repricing
  are covered by exactly-once billing invariants.

### Security

- Token regeneration and affiliate quota transfer now have isolated per-user
  critical rate limits.
- RequestGuard inspects bounded Responses and Claude tool-result text while
  excluding binary media, and blocked or fail-closed requests stop before
  routing, pricing, billing, channel selection, or upstream dispatch.
- Protected fetches reject redirects from a public URL to a private network
  target before issuing the redirected request.

## v1.0.0-rc.1 - 2026-08-15

### Changed

- Product versions became pure RenewAPI SemVer values; product Git tags use
  the `renewapi-` namespace while upstream raw `v*` tags remain untouched.
- The independent RenewAPI prerelease sequence was established at
  `v1.0.0-rc.1` under the product tag `renewapi-v1.0.0-rc.1`.

Release notes are maintained at the product-version level. Do not copy every
commit into this file; use Git history for implementation detail and record
only user-visible behavior, configuration, compatibility, security, migration,
and breaking changes here.
