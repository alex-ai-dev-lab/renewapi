# Current frontend design QA

The active UI is RenewAPI's SnowAPI / Astryx default frontend. Aurora Bento and Obsidian documents describe historical designs; their old PASS results do not validate current code.

- Current deployment and code state: [PROJECT_STATE](docs/PROJECT_STATE.md).
- Current audit and remediation: [UI accessibility and retirement task](tasks/active/2026-10-ui-accessibility-audit.md).
- Contrast checks: `node scripts/qa/color-contrast-check.cjs` after building `web/default`.
- Historical evidence: [Aurora QA archive](docs/aurora-design-qa-history.md).

No blanket acceptance is asserted here. Consult the task's exact checked revision, scenario coverage and outstanding limitations.
