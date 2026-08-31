# Release candidate checklist

Daiku releases are prepared by GitHub Actions and remain drafts until a maintainer approves them. Do not create a tag or release locally.

## Validate the candidate

Run this from the release-candidate commit:

```sh
export GOTOOLCHAIN=go1.26.7
go version
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
make rc-check
git diff --check
```

`make rc-check` verifies formatting, the pinned API checksum and generated client, vet, known Go vulnerabilities, the race-enabled Go suites, OAuth and credential boundaries, redaction, supported command domains, English and Spanish output, agent mode, the Agent Skill, MCP, installer and Homebrew behavior, release version calculation, and macOS/Linux builds on amd64 and arm64.

The Go tests use local HTTP servers and fixtures. They verify that bearer credentials stay within the configured API origin, OAuth secret-bearing requests do not follow redirects, forbidden and foreign-resource responses remain typed and redacted, and commands delegate authorization decisions to the API. They do not prove production RLS policies or a deployed OAuth provider. Those checks belong to the backend deployment review and must be completed without using this repository to mutate production.

GitHub Actions is the release gate. Compatibility tests run with the module minimum, Go 1.25.0. Cross-builds, security scanning, release validation, and release artifact compilation use Go 1.26.7. The workflow validates the release configuration with pinned tooling, creates a GoReleaser snapshot, checks the exact four-archive set against `checksums.txt`, and requires each archive to contain only `daiku`. CircleCI results are not part of candidate approval.

The MCP SDK is pinned to v1.4.1, which contains the fixes for GO-2026-4770 and GO-2026-4773. The stdio regression test for null-suffixed duplicate protocol keys remains as behavioral coverage. Vulnerability allowlists and scanner exceptions are not used.

## Prepare the draft

After the release-candidate pull request is merged and GitHub Actions is green:

1. Open the **Draft release** workflow on `main`.
2. Leave `bump` set to `auto`, set `prerelease` to `rc`, and leave `version` empty unless a maintainer has approved an exact version.
3. Review the computed version in the workflow log before approving any later publication step.
4. Confirm the GitHub release is a draft prerelease and the tag points to the intended `main` commit.
5. Download all four archives, `checksums.txt`, its Sigstore signature and certificate, the SBOMs, and the GitHub provenance attestations.
6. Verify the checksum signature, each archive checksum, the archive's single `daiku` member, and the provenance source digest.
7. Install the candidate with the signed installer and run `daiku version`, `daiku commands --agent`, and `daiku doctor --agent` in a non-production profile.

Publishing the draft or promoting it to stable is a separate human action. A stable promotion triggers the protected Homebrew tap workflow; prereleases never update the tap.
