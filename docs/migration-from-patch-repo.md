# Migration From Patch Repo

Previous build path:

1. Download `QuantumNous/new-api` upstream zip.
2. Apply `newapi-runtime-compat.patch` or `newapi-runtime-compat-with-homepage.patch`.
3. Build image from patched temp tree.
4. Publish tar.gz release assets.

New build path:

1. Build directly from this source fork.
2. Publish multi-arch GHCR image.
3. Release compose examples and checksums.

Legacy patches were removed from the runtime source tree after the source fork became authoritative. They remain recoverable from Git history and the release/audit archive.

| Historical artifact | Git blob | SHA-256 |
| --- | --- | --- |
| `newapi-runtime-compat-with-homepage.patch` | `dd8e05289309c66ce0b36e6dede877d8e9f7d877` | `2048b9970ee156328232d8a1f1b933cb1796d278585677aae135ad37ca5f0522` |
| `newapi-runtime-compat.patch` | `f85de452965141b2f63555549ec48e85c200ae23` | `68ff841c49f7b9c601ab4e91e8df014e784d8c435843b9971a717eb424441d0b` |

To inspect an archived patch without restoring it to the working tree:

```bash
git show <commit>:legacy/patches/newapi-runtime-compat.patch
```
