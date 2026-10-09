# Releasing HTB

HTB releases are created by pushing a version tag such as `v0.4.0`. The
release workflow builds the CLI and server archives, generates checksums, and
publishes a GitHub release.

## Prepare a release

1. Update [CHANGELOG.md](../CHANGELOG.md): move the user-visible entries from
   `Unreleased` under `## [vX.Y.Z] - YYYY-MM-DD` and keep the category
   headings. Write a one- or two-sentence plain-language **Summary** for
   people deciding whether to upgrade, and call out every compatibility change
   in **Breaking changes**.
2. Confirm the supported binaries: Linux `amd64`, macOS `amd64` and `arm64`,
   and Windows `amd64`.
3. Run the checks in [CONTRIBUTING.md](../CONTRIBUTING.md).
4. Commit and merge the changelog update, then create and push the matching
   tag:

   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

The workflow extracts that version’s changelog section with
`scripts/release-notes.sh`, then publishes it as the release body alongside
the archives, `checksums.txt`, and the Linux and Windows installers. It also
adds the installation commands and a link to the complete changelog.

If the changelog has no heading or non-empty **Summary** for the tag, the
workflow fails before a release is created. Amend the changelog on `main`,
then create a new tag; do not silently publish unstructured notes.
