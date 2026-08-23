# Aurora Bento v2 — Dark Mode Design QA

- source visual truth: supplied Aurora Bento v2 Light reference restored from immutable commit `3d91c3fd22fbfae284d48563d5a09ac046a81cd2`
- implementation screenshots: pending GitHub Actions artifact `aurora-dark-visual-qa`
- viewport: `1440 × 1000`
- CSS viewport: `1440 × 1000`
- deviceScaleFactor: `1`
- source pixel dimensions: `1440 × 1000`
- implementation pixel dimensions: pending capture; expected `1440 × 1000`
- state: authenticated Super Admin fixture, `zh-CN`, `Asia/Taipei`, Dark implementation
- source state: Light reference; used for structural/layout fidelity only because the supplied design package has no Dark source board

## Full-view comparison evidence

Pending first Dark browser capture.

## Focused region comparison evidence

Pending first Dark browser capture. Focus regions will include topbar/dock, Bento KPI cards, dense data tables, model cards, and Settings controls.

## Findings

- [P1] Dark source visual truth is not present in the supplied Aurora package.
  - Location: Phase A design target.
  - Evidence: the immutable reference fixture contains the supplied Light Aurora boards; Dark is an intentional derived theme.
  - Impact: exact same-theme pixel fidelity cannot be claimed for Dark. Structural fidelity can be compared against Light, while Dark color/surface quality must be evaluated against the documented Aurora design language and accessibility criteria.
  - Fix: keep the result blocked until browser-rendered Dark evidence has been captured and all actionable P0/P1/P2 implementation findings are resolved; document the source-theme limitation explicitly rather than claiming exact Dark-source parity.

## Comparison history

### Iteration 0

- Earlier findings: Dark mode relied on the generic legacy dark theme and had no independent Aurora reference layer or dedicated browser QA.
- Fixes made: introduced `aurora-dark-reference.css`, wired it into the authenticated shell, and added a seven-page Dark Playwright capture workflow.
- Post-fix visual evidence: pending CI capture.

## Implementation Checklist

- [x] Add independent Dark Aurora tokens and background treatment.
- [x] Wire Dark reference layer into the authenticated product shell.
- [x] Restore the proven seven-page deterministic fixture and switch it to Dark.
- [ ] Capture Dashboard / Channels / API Keys / Logs / Models / Users / Settings at `1440 × 1000`.
- [ ] Check browser console errors and unhandled fixture requests.
- [ ] Compare full views and focused regions.
- [ ] Fix all actionable P0/P1/P2 Dark issues and re-capture.
- [ ] Update `docs/aurora-bento-v2-progress.md` only after evidence is complete.

## Follow-up Polish

Pending capture.

final result: blocked
