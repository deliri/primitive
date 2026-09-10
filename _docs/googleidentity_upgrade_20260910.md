# Googleidentity upgrade — approved for v2026.1.47

User approved bump, commit and push on 2026-09-10. Base revision: 5bd5bcf74e5f8b0b37c2fa532212f05479debcc7. Work moved from /home/d/code/primitive to /work/code/primitive; all 4,554 recorded source files matched. Historical receipts retain their original paths.

## Production and ownership

- Remove seven arbitrary public byte ceilings covering audiences, tokens, command output, access-response whitespace/documents, identity text and JWT headers, and unused responseLimit plumbing. UTF-8, token grammar, exact schema and native lifetime representation remain validated.
- Preserve exact audience spelling in acquisition and verification, including surrounding spaces. Foreign audiences remain refused.
- Validate service-account contexts through Contextstate, containing typed-nil context panics and preserving typed/native identities. Apply the attempt deadline beneath the operation deadline.
- Fix the pinned Google SDK default-universe 2LO path dropping Options.Client: use its explicit auth.Options2LO with Exchange's HTTP client. Google still signs, sends and parses the provider protocol. Its map-shaped additional-claims field is populated from a private typed audience claim; no map crosses Primitive's public contract.
- Fix non-default-universe initialization, previously failing because SDK credential construction supplied neither scopes nor authentication audience. Compose Google's explicit self-signed credentials, authenticated transport and IAM ID-token API through Exchange. These are distinct provider protocols, with no ambient credential discovery or fallback.
- Own credential-file validation in a closed typed service-account document, accepting documented metadata fields and refusing duplicates, unknown members and malformed required facts before HTTP.
- Add temporal.InstantFromUnixSeconds as time.Unix → existing NewInstant, preserving overflow refusal. Googleidentity's timestamp conversion uses Temporal.
- Remove the measured redundant Audience.String validation scan. Its private value is immutable and validated on admission.

Pinned SDK evidence: cloud.google.com/go/auth v0.23.2, credentials/idtoken/file.go and idtoken.go. No dependency-version changes, consumer-repository changes, Witness-source changes, local crypto implementation or new runtime.

## Hostile tests

The entire local testing protocol was read before edits. Its SHA-256 remains 5eaec400ce3eda457f3683fdacac8ec8f4f0f51ee229643b0f7f62ea420e6ec7.

Tables exercise actual metadata HTTP, OAuth HTTP, IAM TLS/authentication and SDK certificate verification; exact signed audience/issuer/key/claim binding; ownership/redaction; receiver preservation; valid inputs beyond former quotas and malformed counterparts; lifetime edges; redirects, duplicates, truncation, empty receipts, closed credentials and no-effect cancellation. Provider and Exchange request counts must both match. Deadline observations use synctest; separate real HTTP tables prove effects. Filesystem fixtures use Filestore; lifecycle checks use channels and Temporal watchdogs.

Nine deliberate defects were killed by tests, not compilation failures: restored audience/token/header quotas, dropped SDK client, ignored attempt deadline, audience trimming, receiver clobbering, replaced signed target audience and Unix-second overflow. Each was restored. The original red-boundaries run also encountered default-client DNS/synctest deadlock after the SDK bypass, not an isolated timing failure. The typed-nil red run panicked. Credential routing separately exposed the non-default-universe SDK initialization failure.

Seven semantic fuzz campaigns passed once each at 30 configured seconds and four workers: **2,393,215 executions**. Public input doors are inventoried against production AST. Oracles include independent UTF-8/token grammar, exact typed response facts, canonical round trips and genuine signed-seed authority. Fuzz cache state is recorded; no empty-cache claim is made. The service-account fuzz fixture has a 64 KiB+1 generation budget, not a production quota.

## Validation

- Googleidentity and Temporal package tests and race tests passed uncached with -count=1.
- Scoped go fix, go vet, staticcheck, errcheck and witness-lint passed. Googleidentity additionally passes errcheck -blank -asserts.
- Production gocyclo <=10 and goconst (minimum four characters, three uses, no tests) passed.
- Full-module production build passed. macOS/arm64 and Windows/amd64 Googleidentity test binaries compiled; native execution was Linux only.
- Full-module tests and remaining global gates were not run.
- Eight before/after benchmarks passed with CPU/memory profiles and retained binaries. [Complete observations](../googleidentity/after_benchmarks.md) include the small slowdowns.

Exploratory failures remain recorded: an overbroad strict Temporal errcheck audit of existing tests, fixture compile errors, missing Temporal architecture allowlisting and Witness diagnostic/shadowing findings. Final scoped checks pass; this is not strict-errcheck cleanup of all older Temporal tests.

## Memory and evidence

Whole tokens, credential files, signed documents and SDK response structures are materialized. This is not an O(1)-memory token or JSON decoder claim. Core's shared endpoint-length and JSON structure limits remain separate Core follow-up work. Schema and native integer representation constraints remain; no package-local byte ceiling remains in Googleidentity.

Raw evidence: /home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910. Per-run receipts preserve commands, Go version, full base commit, source hashes/content snapshots, exit, duration and source-stability checks. measurements.json and mutations.json summarize measurements and deliberate failures; manifest.json/audit.json cover the final review tree and artifacts. Large profiles, binaries and snapshots stay outside Git. Pre-release runs name their exact base-plus-source snapshots, not the later release commit.
