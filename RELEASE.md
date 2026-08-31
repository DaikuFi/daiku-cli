# Release candidate checklist

Daiku releases are prepared by GitHub Actions and remain drafts until a maintainer approves them. Do not create a tag or release locally.

## Validate the candidate

Run this from the release-candidate commit:

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
make rc-check
git diff --check
```

`make rc-check` verifies formatting, the pinned API checksum and generated client, vet, known Go vulnerabilities, the race-enabled Go suites, OAuth and credential boundaries, redaction, supported command domains, English and Spanish output, agent mode, the Agent Skill, MCP, installer and Homebrew behavior, release version calculation, and macOS/Linux builds on amd64 and arm64.

The Go tests use local HTTP servers and fixtures. They verify that bearer credentials stay within the configured API origin, OAuth secret-bearing requests do not follow redirects, forbidden and foreign-resource responses remain typed and redacted, and commands delegate authorization decisions to the API. They do not prove production RLS policies or a deployed OAuth provider. Those checks belong to the backend deployment review and must be completed without using this repository to mutate production.

GitHub Actions is the release gate. The `CI` workflow repeats the checks on Ubuntu and macOS, validates the release configuration with pinned tooling, builds every supported target, creates a GoReleaser snapshot, checks all four archive names and checksums, and requires each archive to contain only `daiku`. CircleCI results are not part of candidate approval.

## Open security policy decision

The CLI explicitly requires `github.com/segmentio/encoding` v0.5.4 and tests null-suffixed duplicate protocol keys through its real stdio MCP transport, addressing GO-2026-4770 without raising the Go requirement. `govulncheck` continues to associate GO-2026-4770 with MCP SDK v1.4.0 because it does not account for the selected transitive parser version; the stdio regression is the behavioral evidence for the fix. The scanner also reports GO-2026-4773, which concerns unauthenticated HTTP MCP servers. Daiku constructs only in-memory and stdio transports and exposes no HTTP MCP handler. Whether that result needs a structural CI assertion or may be accepted as unreachable is the sole open security policy decision. Do not add an advisory allowlist only to make CI green.

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
