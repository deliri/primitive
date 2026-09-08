# Keygen package upgrade — September 8, 2026

Status: reviewed and approved by the user for publication in v2026.1.23.

Keygen remains a bounded Go cryptography adapter. Go owns entropy and Ed25519;
Core owns secret custody, public-key values and stable errors. This slice adds
no provider interface, global production state, key cache, secret store, network
runtime or product policy. Every draw has its existing finite extent. Core's
locked secret copy establishes an active snapshot; destruction invalidates
future projections, while previously exported copies remain caller-owned.

## Production and contract changes

`AdoptPrivateKey` now admits only a consistent standard-library private key.
Previously it discarded a mismatched public half and silently repaired the input.
It now copies the exact bounded input, validates both halves through the existing
adoption boundary, and returns a zero key with `core.ErrKeygenContract` on a
mismatch. An all-zero seed retains `core.ErrKeygenEntropy` and
`core.ErrSecretMaterialAllZero`. Caller bytes remain unchanged on success and
refusal. Invalid persisted keys must be corrected by their owner; this boundary
no longer guesses their intended identity.

The private Go-result adoption helper also checks the private-key suffix against
its separately supplied public result. Its source buffers are cleared on every
return path. Core custody cleanup errors are joined into failed adoption results.
The locally copied seed argument is cleared after Go derives its private key.

Private projection previously derived the same Ed25519 key once for validation
and again for export. The baseline CPU profile attributes about 47% of sampled
CPU to the validation derivation. Private projection now validates the key it
already derived and returns that caller-owned result. The other projections
retain their derivation check. Core's `CopyBytes` owns the locked custody
validation; Keygen checks its narrower seed width after that single snapshot.

The allocation profile separately identified `Ed25519PublicKey.Bytes` as an
allocated slice used only for comparison. Keygen now constructs the comparable
Core-owned public value and compares those public identities directly. It adds
no new Core API and no hand-written cryptography.

## Hostile tests and fuzz oracles

The complete 2,214-line local testing protocol was read before test changes.
Tables expose their own verdicts; the old `prove*` verdict helpers and padded
extent rows were retired. Fixtures own and destroy their secret handles.

| Boundary | Proof |
| --- | --- |
| Private-key adoption | All 512 individual bit changes to one canonical 64-byte private key must refuse; the unchanged control succeeds. Exact extent, seed-only input, adjacent extents, oversized input and canonical/all-zero seeds retain their typed outcomes. |
| Seed adoption | All 256 individual seed-bit positions and a distinct-byte seed preserve the exact seed, Go-derived private/public identity, real Go signature verification and readoption. Zero refuses without a partial key. |
| Entropy reader | Go's `testing/cryptotest` resets the standard-library entropy stream. Exact bytes, count and destination guard bytes match Go; empty/refused calls preserve the next draw. |
| Public tokens and secrets | Every admitted size is compared byte-for-byte with Go after resetting the same stream. Caller mutation cannot alter a later owned projection. Private token tests retain the distinction between legitimate all-zero public bytes and an unissued/oversized value. |
| Uint64 and signing generation | Differential tables use the installed Go reader/Ed25519 implementation as the oracle, including exact big-endian projection. No statistical collision or nonzero guess substitutes for an exact result. |
| Refusal before effects | Invalid typed sizes leave exact zero results and preserve the next Go entropy draw. Private source tables independently attack short/impossible counts, source errors and temporary clearing. |
| Custody | Unset, active and destroyed handles exercise every projection. Copied handles share destruction, exported copies remain independent, and all formatting remains redacted. |
| Forged internal key | Wrong-width seeds are paired with the public key of their padded/truncated prefix, so only the exact-width gate can refuse them. Unset and unrelated public values also refuse with zero projections. |
| Compiler-visible structure | Method-expression witnesses retain value receivers and concrete result types. The five production structs are classified. Embedded-source scans detect extra surface, aliases, imports, maps, local named/generic struct projections and hidden parenthesized calls. |

The deterministic entropy tests use Go's sanctioned `testing/cryptotest` hook
only in serial tests declared through Core's isolation contract. Keygen is the
provider adapter being tested. These tests do not replace `rand.Reader` directly
or expose a test entropy provider in production. Their reference is the installed
Go implementation after resetting the stream; they do not freeze Go's future
entropy-consumption algorithm.

The two external representation doors are `AdoptPrivateKey` and
`AdoptSigningKey`. Each has a compiler-bound fuzz target with a direct semantic
oracle, typed refusal and zero-result proof, exact Go derivation, real signature
verification and canonical round trips. Private-input preservation uses SHA-256
rather than cloning arbitrary oversized fuzz input. Core owns reconstruction of
`ByteCount`; typed size requests and output buffers are not extra JSON parsers.
There is no producer/classifier policy split or durable evidence writer here.
Local entropy/adoption/custody layer triads cover the effects this package owns.

The public adoption regression produces 512 failing leaves against the saved
original production and a passing canonical control. This is one defect across
512 positions, not 512 independent production bugs. The private result helper's
inconsistent-suffix row also fails the saved source. Ten deliberate mutation
experiments fail behaviorally: wrong uint64 byte order, token aliasing, fabricated
zero entropy, skipped temporary clearing, short-read admission, skipped seed
width, skipped public binding (before and after the comparison simplification),
always-refused seed adoption, and hidden parenthesized entropy calls.

## Verification

The final race run passed **1,022 leaf cases** (1,044 test/subtest pass events), with zero failures or skips and **92.7% statement coverage**. Original production had 174 passing leaf cases and 90.7% coverage. Both serial 30-second fuzz campaigns passed: private-key adoption ran 626,665 executions and seed adoption ran 87,979, totaling **714,644 executions**.

Scoped vet, Staticcheck, Errcheck, Nilaway, Witness-lint, go fix diff, Goconst
(minimum 4 characters / 3 occurrences / no tests), and production Gocyclo ≤10
pass. Deadcode reports no Keygen functions; its dependency reachability findings
are retained separately from package-owned defects. Full-module gates were not
run. Darwin/arm64 executes natively with Go 1.27.1. Linux/amd64 and Windows/amd64
compile test binaries; they are not native runtime results.

Coverage is not exhaustive branch proof. Remaining defensive branches include
failures Go's production entropy APIs do not return and failures excluded by
already-validated Core/Go extents. No new public injection API or global
production hook was added to manufacture those states. The architecture scanner
is a local syntax guard, not whole-program semantic analysis.

## Profiled measurements

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| GenerateSigningKey | 42557 | 36285 | 256 | 224 | 5 | 4 |
| GenerateMaximumSecret | 513.4 | 366.9 | 160 | 160 | 2 | 2 |
| AdoptPrivateKey | 41476 | 17219 | 160 | 128 | 3 | 2 |
| SigningKeyValidate | 19982 | 17116 | 64 | 32 | 2 | 1 |
| SigningKeySeed | 20545 | 17055 | 64 | 32 | 2 | 1 |
| SigningKeyPrivate | 41910 | 17628 | 128 | 96 | 3 | 2 |
| SigningKeyPublic | 17572 | 17180 | 64 | 32 | 2 | 1 |
| RandomTokenMaximum | 401 | 322 | 64 | 64 | 1 | 1 |
| EntropyReadMaximum | 280.4 | 277.5 | 0 | 0 | 0 | 0 |
| RandomUint64 | 75.44 | 74.41 | 0 | 0 | 0 | 0 |

Ten fixed workloads ran once per side, requesting 30 seconds each, serially,
with CPU and memory profiles and matching test binaries. Setup and fixture
checks occur before timing; generation includes destruction, private projection
includes clearing the preceding caller-owned buffer, and random draws use the
real production CSPRNG. All baseline test files are restored by Go overlay for
the after pass, so changed test fixtures cannot masquerade as production gains.

All runs use Go 1.27.1, Darwin/arm64, Apple M1 Max, GOWORK=off, AC power at
80% battery and the same ten-way runtime concurrency setting. Effective timed
durations derived from rounded Go output range from 34.56 to 46.55 seconds. Each cell is one sample, not a distribution proving
a timing trend. In particular, unchanged secret generation has appreciably
different before/after timing; its allocation counts remain unchanged. CPU call
paths and removed allocation sites support the retained mechanical changes.

## Evidence and retained attempts

`upgrade_evidence.json` indexes every command, source snapshot, failed attempt,
profile/binary pair and promoted fuzz/cache snapshot by digest. Raw artifacts
stay ignored under `testdata/test-upgrade-20260908`; reports are the reviewable
Git artifacts. This is local engineering evidence, not independent Anvil
acceptance.

Initial harness compilation needed explicit Go public-key conversions; a fuzz
seed needed conversion from the named private-key slice to `[]byte`. Those build
failures do not count as red production evidence. Corrected source overlays
provide the behavioral red runs. Witness-lint caught three weak failure messages
and a disallowed constant alias; the original compiler-bound literal remains.
Go fix modernized two table loops. One redundant type conversion triggered the
exact source inventory and was removed. Every failed attempt is retained.

The final-check script accidentally reused three earlier diagnostic artifact
paths: one coverage file and two cross-compiled test binaries. Rebuilding from
the recorded historical source overlay reproduced all three original SHA-256
hashes exactly. Both generations now have separate digest-addressed retained
copies, and the recovery commands and original capture facts remain visible.
The recorder now refuses existing artifact paths and snapshots requested outputs
by digest. No benchmark profile/binary path was overwritten.

The final audit covers **129 command records, 1,692 source snapshots, 521 artifacts and 2,432 checked digests**, with zero missing or mismatched retained content and no source changes during commands. The three historical artifact relocations are explicit in the manifest; their recovered bytes match the original digests.

Release is next in the recorded usage-priority queue after this package's review.
