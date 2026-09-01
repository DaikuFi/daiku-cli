# Release candidate checklist

Daiku releases are prepared by GitHub Actions and remain drafts until a maintainer approves them. Do not create a tag or release locally.

## Validate the candidate

Run these checks from the release-candidate commit:

```sh
export GOTOOLCHAIN=go1.26.7
go version
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
make rc-check
git diff --check
```

`make rc-check` verifies formatting, the pinned API checksum and generated client, vet, known Go vulnerabilities, race-enabled Go tests, OAuth and credential boundaries, redaction, supported command domains, English and Spanish output, agent mode, the Agent Skill, MCP, installer and Homebrew behavior, release version calculation, and builds for macOS and Linux on amd64 and arm64.

The Go tests use local HTTP servers and fixtures. They verify that bearer credentials stay within the configured API origin, OAuth requests that contain secrets do not follow redirects, forbidden and foreign-resource responses remain typed and redacted, and commands delegate authorization decisions to the API. They do not prove production row-level security policies or a deployed OAuth provider. Complete those checks during backend deployment review without using this repository to mutate production.

GitHub Actions is the release gate. Compatibility tests run with Go 1.25.0. Cross-builds, security scans, release validation, and artifact compilation use Go 1.26.7. CircleCI results are not part of candidate approval.

The release workflow validates its configuration with pinned tooling, creates a GoReleaser snapshot, checks the exact four-archive set against `checksums.txt`, and requires each archive to contain only the `daiku` binary.

The Model Context Protocol software development kit is pinned to v1.4.1, which contains the fixes for GO-2026-4770 and GO-2026-4773. Keep the stdio regression test for null-suffixed duplicate protocol keys as behavioral coverage. Do not add vulnerability allowlists or scanner exceptions.

## Choose the version

Maintainers create drafts from the **Draft release** workflow on `main`. The workflow derives the next version, creates the tag, and builds the draft. A normal release dispatch needs no version override.

`scripts/release/version.sh` computes the version from conventional commits since the latest stable tag:

- A `!` marker or `BREAKING CHANGE` trailer selects a major bump.
- A `feat` commit selects a minor bump.
- Any other change selects a patch bump.
- On a `0.x` line, a breaking change bumps the minor version. Reaching `1.0.0` remains an explicit decision.

Optional workflow inputs can override the calculation:

- `bump` forces `patch`, `minor`, or `major`.
- `prerelease` appends an identifier such as `rc` and continues its existing sequence.
- `version` sets an exact version and ignores `bump`.

Preview the automatic choice locally:

```sh
scripts/release/version.sh --bump auto
```

The command prints the version on stdout and its reasoning on stderr. The workflow refuses to reuse a tag or release a commit range with no changes. Only stable releases reach the Homebrew tap.

## Prepare the draft

After the release-candidate pull request is merged and GitHub Actions is green:

1. Open the **Draft release** workflow on `main`.
2. Leave `bump` set to `auto`, set `prerelease` to `rc`, and leave `version` empty unless a maintainer approved an exact version.
3. Review the computed version in the workflow log.
4. Confirm that the GitHub release is a draft prerelease and its tag points to the intended `main` commit.
5. Download all four archives, `checksums.txt`, its Sigstore signature and certificate, the software bills of materials, and the GitHub provenance attestations.
6. Verify the checksum signature, each archive checksum, the single `daiku` archive member, and the provenance source digest.
7. Install the candidate with the signed installer and run `daiku version`, `daiku commands --agent`, and `daiku doctor --agent` in a non-production profile.

Publishing the draft or promoting it to stable is a separate human action.

## Verify signed artifacts

The installer supports macOS and Linux on amd64 and arm64. It verifies the Sigstore identity on the signed checksum manifest and then verifies the selected archive's SHA-256 checksum. It installs atomically to `~/.local/bin/daiku`, restores an existing binary after interruption, and never invokes `sudo`.

Download `scripts/install/daiku.sh`, inspect it, install [cosign](https://docs.sigstore.dev/cosign/system_config/installation/), and run the local file with an exact v-prefixed semantic version:

```sh
DAIKU_VERSION=v0.1.1-rc.1 sh ./daiku.sh
```

Set `DAIKU_INSTALL_DIR` to use another user-writable destination.

The installer serializes updates with `$DAIKU_INSTALL_DIR/.daiku.install.lock`. If that directory remains after an interrupted process, confirm that no Daiku installer is active before removing it. Inspect `.daiku.install.*` and `.daiku.backup.*` leftovers under the same condition. The installer never deletes a lock or leftover created by another process.

GitHub records SLSA build provenance for every archive, software bill of materials, checksum, signature, and certificate. Verify a downloaded candidate against this repository's workflow:

```sh
gh attestation verify daiku_0.1.1-rc.1_linux_amd64.tar.gz \
  --repo DaikuFi/daiku-cli \
  --signer-workflow DaikuFi/daiku-cli/.github/workflows/release.yml
```

Add `--format json` during an audit and compare the provenance source digest with the commit recorded in the draft release.

## Publish to Homebrew

The **Publish Homebrew tap** workflow runs when a maintainer promotes a draft release to a published stable release. It re-verifies the Sigstore signature on `checksums.txt`, regenerates the formula from the signed manifest, and commits `Formula/daiku.rb` to the tap. It refuses prereleases and avoids committing an unchanged formula.

Preview a formula without changing the tap:

```sh
scripts/release/homebrew.sh VERSION dist/checksums.txt dist/daiku.rb
```

The workflow requires one-time configuration outside this repository:

- A public `DaikuFi/homebrew-tap` repository, because Homebrew fetches taps anonymously.
- A repository environment named `homebrew-tap`, preferably with required reviewers.
- A `HOMEBREW_TAP_TOKEN` environment secret. Use a fine-grained personal access token scoped only to `DaikuFi/homebrew-tap`, with read and write access to repository contents.

Use the workflow's manual dispatch to republish the formula for an already-published stable version, such as after correcting its template.
