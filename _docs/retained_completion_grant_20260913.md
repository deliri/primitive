# Retained completion grant — v2026.1.85 candidate

This is a scoped contract change, not a full Submission/Submissionauth sweep or
an independent acceptance receipt. The API owner authorized the extension and
published dependency update. Product policy selects the provider; Primitive
authenticates typed facts and executes provider mechanics.

## Contract

`GrantRecord` retains only the original authority-signed `GrantPayload` and
attestation. Issuer and receiver `Record` methods first validate the existing
capability binding. The retained record contains no spendable URL or headers.
Its JSON admission bounds one fixed-size agreement to 4096 bytes; this is not a
limit on object bytes, corpus size, manifest size, or total evidence volume.

`CompletionExpectation` and credentialed `CompletionVerification` now require
that record and an explicit, independently selected provider. There is no
compatibility constructor accepting the old bearer-bearing grant. Signature
authentication still precedes cross-document binding. A completion proof is
not provider custody: the API must separately observe the exact generation,
verify object bytes, recheck account authority, and persist its own receipt.

## Proof surfaces

- `TestGrantRecordRetainedAgreementLayerTriad`: real local HTTP upload, real
  grant/completion issuance, exact serialization and verification, no bearer
  projection, missing agreement, foreign and unset provider, unsigned time.
- `TestGrantRecordFreshProcessLayerTriad`: test-owned disk file read by a fresh
  OS process, independently pinned authority/device keys, exact authenticated
  completion, unsigned issue time, absent grant, truncated record. The child
  does not receive the original bearer. This is **not** the API Firestore writer
  and does not claim its manifest, restart, or finalization behavior.
- `TestGrantRecordEverySignedFactMutationRefusesCompletion`: known authentic
  baseline followed by exactly one changed request commitment, authorization,
  capability commitment, issue/expiry/retention instant, signer, signed extent,
  digest, or signature. Every change stays nominally valid, actually differs,
  and must produce a typed authentication refusal and zero completion proof.
- `TestGrantRecordMalformedInputPreservesBothReceiverStates`: malformed,
  truncated, duplicate, missing, unknown/bearer, and scalar field input cannot
  replace either a populated or zero receiver.
- `TestGrantRecordByteBoundaryLayerTriad`: below/at/above input ceiling and
  absent issuer/receiver projections; rejected projections remain zero.
- `TestRetainedGrantCredentialedCompletionLayerTriad`: local credentialed
  verifier independently proves exact facts, absence and provider refusal.
- Existing completion hostile tests now use the clean-break retained contract;
  request/capability/build/nonce/content bindings and authentication order are
  still behavioral requirements, not compatibility fixtures.
- Exact struct conversion pins no bearer field. The package struct inventory
  classifies the new protocol fact; the ingress inventory names its fuzz target.

Two semantic fuzz targets cover external record admission and guaranteed
structurally valid signed-fact recombinations. They use real typed issuer seeds,
the production decoder, canonical closure, independent pinned trust, and zero
proof on refusal. Neither treats Validate or absence of panic as authentication.

The record/verifier is not a batch producer-to-classifier handoff. No 50-row
classifier matrix is claimed. The new rows are distinct contracts, not a claim
that their count independently meets every package-wide 10/10/20 sweep floor.
The older package-wide tests remain; a full sweep's earned-row audit is not
closed by this slice. No downstream API writer, manifest, reporter, CLI, cloud
generation readback, or independent authority acceptance is claimed here.

## Retained local execution facts

Evidence directories are under
`/private/tmp/peachfuzz-state-readonly-evidence/`. Each directory has retained
stdout/stderr, source snapshots, source hashes before/after, exact argv, base
revision, dirty fact, toolchain, exit and artifact byte counts/digests in
`execution.json`. Counts include parent tests. Missing unavailable/not-run
identities are explicitly null, not zero and not passed. These are local
diagnostics, not independently validated acceptance receipts.

Base revision: `544d71ab685f24fabe68a5133129b569d1e8b3bb`.

Final development phases:

- `primitive-retained-grant-layer-tests`: both packages, uncached, 959 pass
  events, no failed/skipped events; exit 0.
- `primitive-retained-grant-race`: both packages, race, fixed shuffle 732491,
  count 1, 959 pass events, no failed/skipped events; exit 0.
- `primitive-retained-grant-final-tools-vet` and `...-staticcheck`: exit 0.
- `primitive-retained-grant-final-tools-doctrine`: exit 1, four newly added
  diagnostics lacked observed values. `primitive-retained-grant-doctrine-context`
  retains the corrected run, exit 0; the first result is not discarded.
- `primitive-retained-grant-provider-mutation`: deliberately disabled the
  provider comparison; both verifier layers failed (4 fail events including
  parents). `primitive-retained-grant-provider-restored`: exact check restored,
  same filtered scope, 10 pass events. The mutation is not in production.
- Earlier `primitive-retained-grant-auth-mutation` removed grant authentication
  and substituted another proof; unsigned issue time was incorrectly accepted.
  The failure and `primitive-retained-grant-auth-restored` remain retained.
- `primitive-retained-grant-completion-benchmark`: observable exact completion
  verification, fixed real-transfer fixture outside timing, 3-second target,
  31,155 iterations, 118,378 ns/op, 12,969 B/op, 241 allocs/op. One local sample,
  no baseline comparison, no requested profiles, no performance trend claim.
- `primitive-retained-grant-semantic-fuzz-final` and
  `primitive-retained-grant-recombination-fuzz-final`: serial targets, one worker,
  GOMAXPROCS=1, 2-second exploration plus at most 1-second minimization budgets;
  both exit 0. Existing fuzz cache was eligible. Generated interesting corpus
  churn is not promoted durable regression evidence. No target failure occurred.

Earlier compilation and inventory failures are retained separately, including
`primitive-grant-record-compiler-break`, `...-callers`, `...-focused`,
`...-restart-first`, and `primitive-grant-record-signed-facts-first` (typed
ByteCount incorrectly used as arithmetic). The corrected typed constructor run
is `primitive-grant-record-signed-facts-typed-count`. These failures are not
folded into later passes. Clean committed-revision checks and external acceptance
must be reported separately from the development phases above.
