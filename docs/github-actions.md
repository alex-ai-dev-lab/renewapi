# GitHub Actions

The `build-release.yml` workflow is the source and product release gate. It:

- runs the Go, frontend, module-integrity, database transaction, race, vet, and
  build checks before the image job can start;
- accepts an optional `base_sha` so changed-file checks can cover the complete
  reviewed range instead of only the final commit;
- builds and pushes `linux/amd64` and `linux/arm64` images to GHCR;
- publishes immutable `sha-<12 character commit>` tags and records the image
  digest in the release notes;
- generates an amd64 tarball with both Compose examples and a portable
  `default.env.example`;
- downloads the uploaded artifact again and validates `CHECKSUMS.txt`, so the
  delivered package itself is checked;
- when release creation is enabled, pins the release tag to the workflow commit
  and verifies the resulting tag before the job succeeds.

Event/channel behavior:

- `main` pushes publish `edge` and immutable `sha-<12 character commit>` images
  and do not create a product GitHub Release.
- `renewapi-vX.Y.Z-rc.N` pushes validate
  `stripPrefix(gitTag, "renewapi-") == VERSION`, publish the exact Docker
  version tag without its leading `v`, `rc`, and SHA image tags, and create a
  `RenewAPI vX.Y.Z-rc.N` prerelease without moving stable aliases.
- `renewapi-vX.Y.Z` pushes validate the same prefix rule, publish
  version/minor/major/`latest`, and create the stable `RenewAPI vX.Y.Z`
  Release.
- Raw upstream `v*` tags do not match the workflow trigger and cannot enter the
  product release path.
- Manual dispatch retains the source-image packaging path. Its source release
  is explicit and cannot publish reserved `edge`, `rc`, or `latest` aliases.

The Docker build receives the product version from `VERSION`; build channel and
commit are independent metadata. A real hosted Actions run is still required
to prove GHCR alias and Release behavior.

Set `base_sha` to the reviewed source baseline when a release contains multiple
commits. Deploy an immutable SHA tag or digest, not a mutable tag. The workflow
does not download upstream source archives and does not run `git apply`.
