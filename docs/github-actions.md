# GitHub Actions

The manual `build-release.yml` workflow is the source release gate. It:

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

Set `base_sha` to the reviewed source baseline when a release contains multiple
commits. Deploy an immutable SHA tag or digest, not a mutable tag. The workflow
does not download upstream source archives and does not run `git apply`.
