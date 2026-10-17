# Contributing

Before changing anything, read [`.github/claude/WRITER.md`](.github/claude/WRITER.md) for the repo invariants and the definition of done. Every PR needs a `CHANGELOG.md` entry under `## [Unreleased]`.

## Releasing

1. **CHANGELOG**: rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` (date the section) and add a fresh empty `## [Unreleased]` above it.
2. **Version reference**: no version-bump commit is needed. The version is injected at build time through ldflags (`-X github.com/shapedthought/owlctl/cmd.version=<tag>`, plus `cmd.commit` and `cmd.date`), and local builds report `dev`. The `release.yml` change that passes these ldflags lands in a separate PR (#182); until it merges, release binaries still report `dev`.
3. **README**: add a row for the new version to the "Change Log" table in `README.md`. Use a bold headline and a short summary taken from `CHANGELOG.md` only.
4. **Tag and release**: push an annotated tag, then create the release from it. Do not create a draft: the release workflow runs on `release: created`.

   ```bash
   git fetch origin
   git tag -a vX.Y.Z <release-prep-merge-sha> -m "owlctl vX.Y.Z"   # tag the merge commit of the release-prep PR
   git push origin vX.Y.Z                                           # pushing a tag does not trigger the release workflow
   sed -n '/^## \[X.Y.Z\]/,/^## \[/p' CHANGELOG.md | sed '1d;$d' > /tmp/notes.md
   gh release create vX.Y.Z --verify-tag --title "vX.Y.Z — <headline>" --notes-file /tmp/notes.md --latest   # NOT --draft
   ```

5. **Post-release checks**: confirm the workflow completed and the release has the `.tar.gz`/`.zip` assets and `.sha256` files for every platform. Run `scripts/check-release-urls.sh`. Also test the install: run the template's install step for the new tag and for `latest` (download, sha256, extract, `--help`), and check that `releases/latest` redirects to the new tag.
6. **Pipeline template**: bump the default `owlctlVersion` in `examples/pipelines/templates/install-owlctl.yml` to the new tag (it is defined only there).
