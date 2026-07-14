# GitHub Actions

The repository builds and validates the checked-in source directly. Workflows do
not download an upstream archive or apply patch files.

## Continuous Integration

`.github/workflows/ci.yml` runs for pull requests and pushes to `main`:

- changed Go files must pass `gofmt`;
- the complete Go repository runs tests, vet, and build;
- both frontends install from their lockfiles and build;
- the default frontend also runs tests and type checking;
- ESLint, Prettier, and copyright checks apply to changed frontend files;
- Docker builds only after the backend and both frontend jobs pass.

Changed-file formatting and lint checks prevent new debt without making the
workflow fail on unrelated historical files. Full tests, type checking, and
builds remain mandatory.

## Image Build And Release

`.github/workflows/build-release.yml` is manually dispatched after a reviewed
commit is pushed. It:

- validates MySQL and PostgreSQL channel configuration transactions;
- runs the complete backend and frontend quality gates;
- builds and pushes `linux/amd64` and `linux/arm64` images to GHCR;
- publishes immutable `sha-<12 character commit>` tags;
- records the manifest digest as a workflow output;
- generates an amd64 tarball artifact and optional GitHub release.

Set `base_sha` to the reviewed source baseline so changed-file checks cover the
entire delivery range. Deploy the immutable SHA tag or digest, not `latest`.
