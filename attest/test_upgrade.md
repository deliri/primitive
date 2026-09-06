# Attest test upgrade — 2026-09-06

Scope: attest tests, fuzz targets, benchmarks, and local evidence. No production
Go files were changed. The entire project-local
[testing protocol](../_docs/testing_protocol.md) was read before editing tests.
This is a contract ratchet over existing behavior, with deliberate production
mutations compiled through Go overlays for red-state proof.

## Boundary philosophy

Attest proves the blind mechanics between caller-owned structs: bounded
canonical bytes, typed domain separation, signature authenticity against the
caller-supplied trust set, owned storage, and exact returned evidence. Fixtures
supply their own domains and trust inputs. The tests do not assign product
meaning, authority policy, completion, or human acceptance to those values.
Malformed JSON strings and byte sequences are deliberate external-input fixtures;
admitted values are compared as typed facts through the shared Go agreement.

## Execution-layer requirements

Primitive's operations must stream with bounded working memory. Reading and
hashing N bytes necessarily takes O(N) time; the working memory must not grow
with N. Benchmarks compare 64 KiB and the owner-defined 1 MiB maximum with one
reused 8 KiB buffer. They exercise the standard library's bytes.Reader/io.Copy,
SHA-256 and Ed25519 paths. No replacement hash, cryptographic implementation,
JSON parser, or production compatibility layer was introduced.

The scalar zero-allocation assertion moved out of benchmark setup into a real
allocation ratchet with the compiler-owned runtime-allocation isolation
declaration. A one-allocation mutation is rejected by that test.

## Surfaces and proof

| Public surface | Hostile/runtime proof | Semantic fuzz proof |
| --- | --- | --- |
| Sign: body callback and bounded stream | Canonical-body matrix, terminal-exit matrix, producer triad, independent SHA-256 facts | FuzzSignCanonicalBodyStreaming |
| Sign: local Ed25519 material | Private-key boundaries and caller-storage isolation | FuzzSignLocalPrivateKey |
| Sign: external signer capability | Standard-signer matrix; callback counts; no signing after body failure | FuzzSignExternalSignerResponse |
| SigningDomain callback / domain text | Public text boundary matrix, reconstruction failure matrix, self-reference tests | FuzzSigningDomainAdmission; FuzzEnvelopeJSONSemanticClosure |
| NewTrustedKeys / TrustedKeysRequest.Validate | Cardinality, duplicates, all-zero anchor, owned storage and caller mutation | FuzzTrustedKeysExternalAdmission |
| SignRequest.Validate / VerifyRequest.Validate | Explicit observation that validation performs no canonical writes or signing | Sign/domain/private-key targets and verification mutation target |
| Verify / Verified | Mutation matrix, verifier triad, exact retained envelope, zero-proof rejection | FuzzVerifyRejectsEveryIndependentlyMutatedSignedField; signed JSON and streaming targets |
| Envelope.UnmarshalJSON | Normalization, hostile rejection with receiver preservation, exact whitespace ceiling | FuzzEnvelopeJSONSemanticClosure |
| Envelope.MarshalJSON / Validate | Canonical extent/order, schema and projection triads | Envelope JSON and domain targets |
| Signature.UnmarshalJSON | Canonical and hostile receiver matrices; independent JSON/hex facts | FuzzSignatureExternalJSONDoor |
| Signature.Bytes / Hex / MarshalJSON / Validate | Zero-value refusal without partial output, exact projections | Signature JSON target checks exact decoded bytes and canonical fixed point |
| CanonicalObject scalar methods / End | Grammar, typed values, escaping/closing-byte extent, terminal states, no heap allocation | FuzzCanonicalObjectEmitsStrictlyDecodableAndStableDocuments |
| CanonicalObject.Value | Hostile owner callbacks, typed first-refusal preservation, callback suppression | FuzzCanonicalObjectNestedMember |

The existing exact public-surface/alias test blocks unnoticed new entry points.
The existing compiler-referenced JSON-door inventory and production-struct
inventory remain. The package has cryptographic admission and verification,
not a producer→classifier classification lattice, so the 50-case classifier
matrix rule is not applicable. It owns no durable writer, manifest store,
reporter, CLI or network service; those layers are not exercised or claimed.

## Weak contracts retired

- The canonical-object fuzzer's loose-map branch is gone. Every generated name
  and scalar participates in an exact standard-library token projection, and
  the independent grammar oracle rejects spurious refusals as well as bad
  acceptances.
- Signature JSON fuzzing now compares input-derived bytes using Go's JSON and
  hex decoders; self-round-tripping alone could accept silent byte changes.
- Verification mutation fuzzing proves that its authentic baseline succeeds
  before each independently checked mutation is rejected.
- Envelope JSON seeds now include production-emitted documents at the accepted
  byte ceiling and one below/above it, duplicate/missing members, uppercase
  signature material, and trailing data. Canonical seed refusal fails the oracle.
- Canonical-body and builder signing tests now check the actual SHA-256 digest,
  rather than checking only length or only successful verification.
- Partial failure/panic, ignored overflow, late writes, exact callback counts,
  and validation-versus-execution are observable typed assertions.
- Duplicate member-name cases and shape-validation rows that varied unread
  payloads were removed, along with their unused fixture helpers.
- Signature sizes derive from crypto/ed25519 and encoding/hex constants.
  The independent frame oracle derives its prefix from its existing fixed
  golden vector rather than copying the production generation literal again.
- Benchmarks retain and validate their output after timing. The signing fixture
  reuses its chunk; verification and JSON encode/decode now have benchmarks.

## Red-state evidence

[First mutation batch](testdata/test-upgrade-20260906/mutations.json) and
[additional batch](testdata/test-upgrade-20260906/mutations-additional.json)
retain exact replacements, filters, exits, and results. Source overlays and
individual stdout/stderr records are retained beside them. Twelve semantic
mutations failed: wrong scalar value, wrong digest, lost trust key, disabled
verification, unclosed writer, disabled external-signer verification, altered
signature bytes, callbacks after refusal, bad domain admission, inconsistent
private key, bypassed nested JSON admission, and an introduced scalar allocation.

One mutation survived: removing the redundant guard in CanonicalObject.fail.
The earlier ready() gate already prevents later callbacks from reaching fail,
so that mutation does not change the exercised public behavior. It is retained
as a surviving, non-load-bearing mutation, not counted as a killed mutation.
Removing the actual ready() gate makes the callback-suppression test fail.

## Review corrections after the initial green runs

The initial green runs were insufficient. The subsequent review found and
corrected these specific gaps before freezing the reviewed source:

| Gap | Replacement proof |
| --- | --- |
| Internal domain fixture rejected malformed text using production's validator before the tested boundary ran | The fixture now admits the representation; disabling production admission makes the round-trip table fail |
| Accepted body/domain/key rows could check only nil errors or partial facts | Exact body digest, extent, domain and signer checks; independent Ed25519 frame verification; retained proof checks |
| Signature separation checked whole envelopes, whose other fields already differed | The signature itself must separate, and both signatures must verify against independent frames |
| Envelope fuzzing could accept a consistently corrupted round trip | Standard-library decoding derives every expected field from the original bytes |
| Nested JSON fuzzing lacked an independent admission oracle | Bounded recursive standard-library token walk; pairwise Unicode EqualFold checks; depth, field, item and enclosing-byte boundaries |
| Late object member test could pass because the second End always refused | Observe owner callbacks and the complete caller backing storage before the second End |
| Maximum member-count test checked only nil error | Check every name, value and order against standard-library token encoding at maximum-1/exact/+1 |
| Trust-isolation rows verified only the first admitted key | Replace every caller slot; verify every original key and reject the replacement key |
| Signature rows changed key/body without a distinct parser boundary | First-byte-only and last-byte-only sentinels with independent decoded-byte checks |
| Shape-validation rows claimed execution was deferred without proving the later outcome | Execute the admitted request and check the exact verification refusal or retained proof |

The redundant standalone retained-writer test and weaker first-error test were
removed after the stronger terminal-exit and typed-refusal tests covered them.
The [review mutation batch](testdata/test-upgrade-20260906/mutations-review.json)
contains six additional failing mutation runs. The [boundary mutation batch](testdata/test-upgrade-20260906/mutations-boundary.json)
adds closing-byte overflow and whitespace-before-limit-check mutations. Across
the four batches,
20 mutation executions are killed; two deliberately repeat earlier mutations
against different strengthened oracles, so this is not a claim of 20 distinct
production defects.

The first doctrine-lint run reported two diagnostics lacking observed values.
Those messages were corrected, and the failing attempt remains in the evidence.

## Validation and measurements

The final unfiltered package run passes: 626 test/subtest/fuzz-seed pass
events, zero failures or skips, and 93.1% statement coverage. Counts describe
execution accounting, not independent proof cases or a quota-based quality claim.
Coverage does not establish that constructor-unreachable defensive branches have
public-path proof.

All ten semantic fuzz targets completed their initial 30-second runs. The envelope and nested-member
JSON oracles changed during review and each completed a further recorded
30-second run. Signature fuzzing received document-limit seeds and was rerun
in the final boundary phase. The other seven targets were unchanged. The initial race/shuffle
run and package vet/staticcheck/witness-lint runs passed on the earlier source;
they are historical results, not validation of later edits. Further gates are
deferred until every package is upgraded, as the user instructed.

Earlier benchmark passes preceded the completed row and byte-boundary audits;
they remain intermediate attempts. The final boundary phase follows
[boundary-tests.json](testdata/test-upgrade-20260906/boundary-tests.json), runs the
changed signature fuzz target, then serial profiled benchmarks last. Its exact
plan is [boundary-phase-plan.json](testdata/test-upgrade-20260906/boundary-phase-plan.json).
The last byte-boundary additions prove that whitespace counts toward Signature's
shared JSON cap and that caller-owned destination prefixes count toward the
complete canonical-object cap, each at maximum-1/exact/+1.
See [before](before_benchmarks.md) and [after](after_benchmarks.md) for workload
measurements and interpretation. All attempts, including diagnostic failures
and the surviving redundant-guard mutation, remain in the local bundle.

The validation runs used the recorded dirty workspace, before the user-approved
commit. The commit contains the tests and package reports. Full raw records,
profiles, binaries, mutation overlays and manifests remain in the ignored local
`testdata/test-upgrade-20260906/` execution bundle. These are local review facts,
not an independent acceptance receipt for a committed revision.
