# Aurora Bento v2 Release Acceptance Record

Status: **FORMAL ACCEPTANCE IN PROGRESS**

This record distinguishes automated release-candidate evidence from manual/environment acceptance. It must never be interpreted as a claim that desktop screen-reader, literal browser 200% zoom, or external production deployment checks have passed unless those checks are explicitly recorded as PASS.

## Source

Acceptance record initialized from `main` at:

- main SHA before acceptance documentation: `dfb43330a7b7a6bd5cd5544e274cece2ecd4f7fa`
- last browser-validated product-code SHA: `b2c73729c170d619d95170b4234804b813866710`
- comparison `b2c7372..dfb4333`: documentation-only (`docs/aurora-bento-v2-progress.md`); no product code changed

The authoritative release source is always the final `main` SHA after release-hygiene/documentation work stops. If that SHA contains product-code changes after the product-code SHA above, the prior browser evidence is not sufficient and the affected automated gates must be refreshed.

### Exact-SHA evidence rule

The authoritative final-head Secondary proof is the `aurora-secondary-surfaces` commit status attached to the frozen final `main` SHA. Its target Action run and the uploaded artifact's `run.txt` bind `GITHUB_SHA` and `GITHUB_RUN_ID` without creating a self-referential documentation commit after the run. Do not move `main` merely to paste an Action run number into this file; doing so would create a new HEAD that the pasted run did not validate.

## Automated evidence baseline

Last completed product-code Secondary evidence before formal acceptance:

- workflow: `Aurora Secondary Surfaces QA`
- run: `32703725766`
- job: `97360351957`
- product SHA: `b2c73729c170d619d95170b4234804b813866710`
- artifact: `9511597674`
- artifact digest: `sha256:79c4f2ab67ed267a096895e76a6ea12aee5f845d345ebbd05275bfacadd9f12e`
- public cases: `13`
- authenticated cases: `5`
- themes: `2`
- viewports: `2`
- total audits: `72`
- failures: `0`
- console errors: `0`
- page errors: `0`
- horizontal overflow: `0`
- unexpected HTTP 4xx/5xx: `0`
- backend HTTP baseline: `200 × 635`
- 429: `0`
- ChunkLoadError: `0`
- audit-exception: `0`

Formal acceptance requires an exact-final-head Secondary run after all repository hygiene/documentation commits are in place. The final run ID, job ID, artifact ID, artifact digest, and matrix summary are release evidence attached to that final SHA.

## Build & Release quality gate

`.github/workflows/build-release.yml` is the full repository release-quality pipeline. A release-candidate validation must use the exact frozen source SHA and must not create a product GitHub Release during validation.

Required quality coverage includes:

- immutable source SHA check;
- Compose template validation;
- changed Go formatting check;
- changed JSON-boundary policy check;
- Go module integrity (`go mod verify`, `go mod tidy -diff`);
- tracked Go package capture;
- default frontend install/tests/typecheck/build;
- changed default frontend ESLint/copyright/Prettier checks;
- classic frontend build;
- full tracked Go tests;
- selected Go race tests;
- `go vet`;
- production Go builds;
- commit diff check;
- MySQL channel transaction and RequestGuard migration checks;
- PostgreSQL channel transaction and RequestGuard migration checks;
- multi-arch image build with provenance/SBOM;
- linux/amd64 image export;
- release-asset checksums and re-download verification.

A manual `workflow_dispatch` source build with `create_release=false` still publishes GHCR image tags; it is not side-effect-free. Use a non-reserved validation tag when manual dispatch is used, never `latest`, `rc`, or `edge`. Product release creation remains disabled for RC validation.

## Release convention and current identity

Repository policy is authoritative:

- `VERSION` contains the pure product version, currently `v1.0.0-rc.3`.
- product Git tags use `renewapi-v<version>`.
- prerelease product tag example: `renewapi-v1.0.0-rc.3`.
- `main` publishes `edge` plus immutable `sha-<short-sha>` image identities and does not create a product GitHub Release.
- prerelease product tags publish the version tag, `rc`, and immutable SHA image identities; they do not update stable `latest`.
- stable product tags may publish version/minor/major/`latest` aliases after validation.
- historical tags and releases are immutable and are not rewritten.
- product tag validation requires the tag-derived version to equal `VERSION`, the tag to resolve to the checked-out exact HEAD, and the working tree to be clean.

Important current release-identity fact:

`renewapi-v1.0.0-rc.3` already exists and was released from commit `a3cfa2bd35998a665d912d87ffd19b09600c3620`. Therefore the current later `main` cannot be formally published again as `renewapi-v1.0.0-rc.3`, and the existing tag/release must not be moved or rewritten. A future formal product release requires a separately prepared, repository-approved new `VERSION`/tag identity. This acceptance task does **not** choose or create that next version.

## Automated DoD

Baseline product-code automated DoD:

- [x] Desktop Light automated/browser evidence
- [x] Desktop Dark automated/browser evidence
- [x] responsive baseline
- [x] core deep-state coverage
- [x] Settings real-backend mutation/destructive/persistence coverage
- [x] Advanced Settings real-backend coverage
- [x] Secondary 72-case browser matrix
- [x] Secondary axe actionable violations = 0
- [x] Secondary console errors = 0
- [x] Secondary page errors = 0
- [x] Secondary horizontal overflow = 0
- [x] Secondary unexpected HTTP 4xx/5xx = 0
- [x] Secondary 429 = 0
- [x] Secondary ChunkLoadError = 0
- [x] Secondary audit-exception = 0
- [x] static assets excluded from the web document-rate-limit budget while application/document limiting remains enabled
- [ ] exact frozen final-main Secondary evidence — must be attached to the final SHA after acceptance/hygiene commits
- [ ] exact frozen final-main Build & Release quality validation — must complete before automated RC acceptance

## Manual DoD

The executable procedure is `docs/aurora-bento-v2-manual-acceptance.md`.

- Literal browser UI 200% zoom — **PENDING MANUAL**
- NVDA — **PENDING MANUAL**
- VoiceOver — **PENDING MANUAL**
- JAWS — **PENDING MANUAL / optional when environment unavailable**
- external deployment/restart smoke — **PENDING ENVIRONMENT**

## Release decision

Current decision: **NOT READY FOR FORMAL PRODUCT RELEASE**.

Reasons:

1. final frozen-main automated RC gates still need exact-SHA evidence after acceptance/hygiene commits stop;
2. manual/environment gates remain pending;
3. the current `VERSION` identity `v1.0.0-rc.3` is already consumed by an immutable historical tag/release on an older commit, so the current later main has no prepared unused product-release identity.

Passing automated gates may advance this to **AUTOMATED RC READY — manual/environment acceptance pending**, but it does not by itself authorize a product tag or a `FULL RELEASE PASS`.