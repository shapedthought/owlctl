# Contributing

Before changing anything, read [`.github/claude/WRITER.md`](.github/claude/WRITER.md) for the repo invariants and the definition of done. Every PR needs a `CHANGELOG.md` entry under `## [Unreleased]`.

## Releasing

1. **CHANGELOG**: rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` (date the section) and add a fresh empty `## [Unreleased]` above it.
2. **Version reference**: bump the version string in `cmd/utils.go` (the update check). This step goes away once #182 lands.
3. **README**: add a row for the new version to the "Change Log" table in `README.md`. Use a bold headline and a short summary taken from `CHANGELOG.md` only.
4. **Tag and release**: push an annotated tag, then create the release from it. Do not create a draft: the release workflow runs on `release: created`.

   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   gh release create vX.Y.Z --verify-tag
   ```

5. **Post-release checks**: confirm the workflow completed and the release has the `.tar.gz`/`.zip` assets and `.sha256` files for every platform. Run `scripts/check-release-urls.sh`.
6. **Pipeline template**: bump the default `owlctlVersion` in `examples/pipelines/templates/install-owlctl.yml` to the new tag (it is defined only there).
