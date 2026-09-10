# Proofledger upgrade — v2026.1.50, 2026-09-10

Base: a40accdad80780af6ab79d65e4b702e5bc3ee2e6 (v2026.1.49), /work/code/primitive on Furnace. The entire local testing protocol was read before the test changes. The current streaming rule supersedes the older quota examples in that protocol.

## Behavior

Removed event, receipt, signed-receipt and page JSON extent quotas, including their exported constants and obsolete helper paths. Valid events above the former 96-KiB quota now issue, encode, decode and replay. Receipt JSON admits whitespace beyond the former 4-KiB and 64-KiB quotas without changing the typed value. Page validation checks the actual event chain without serializing the complete page just to measure it. Exact canonical bytes are still mandatory at DecodeEnvelope; receipt decoders retain their existing whitespace tolerance. Typed framing, identity, payload validation, hash linkage and signature verification remain mandatory.

NewGenesisHead now returns a zero head on refusal. NewEnvelope also explicitly returns zero if final validation refuses. Sequence.Next retains both the sequence-conflict and package-contract identities at uint64 exhaustion. A page ending at the maximum sequence cannot claim More=true: the next sequence does not exist. A final page at that sequence and an empty suffix remain valid.

The eight-event pagination window remains a per-page working window, not a total-ledger quota. Replay retains only its current Head and can process any representable number of events. Envelope and payload JSON APIs carry complete typed values and byte slices: allocation is proportional to the current encoded payload, not O(1) relative to an individual event's extent. No streaming JSON transport or terabyte transfer is claimed. Tests reach a 1-MiB-plus-17-byte event and receipt framing; a single event also exceeds the former page byte quota. Receipt WriteCanonical has fixed typed content and preserves partial progress, native writer failures and io.ErrShortWrite.

## Proof surfaces

| Surface | Evidence |
|---|---|
| Event issuance/hash/JSON | Per-field commitment mutations, typed missing/contradictory facts, below/at/above former quotas, large canonical round trips, malformed counterparts, zero-output refusal |
| Replay/finalization | Real NewEnvelope → MarshalJSON → DecodeEnvelope → Verifier.Observe/Finish chain; resume, duplicate, gap, reversal, authentic sibling link, tampering, empty prefix/suffix, truncation and native sequence exhaustion |
| Pagination | Exact continuation and cursor identity, per-page cardinality, empty result, missing/reordered events, large event, impossible More at native exhaustion |
| Receipt binding/authentication | Local unsigned event-to-receipt binding checks and independent signed-document verification; altered event/request/sequence/hash/predecessor/time, foreign signature, producer mismatch, absent event/document/trust and zero verified output |
| Canonical writer | Exact bytes, partial/full progress with native error, short write, absent receipt, nil destination |
| Identity/numeric/domain ingress | Existing identity/number codecs, exhaustive uint16 PageLimit and byte signing-domain domains, native sequence boundaries, receiver preservation and semantic round trips |
| Compiler structure | Production data-flow roles retained; ten explicit JSON/text decoder coordinates bound to concrete functions; five fuzz targets compiler-bound |

The five fuzz targets cover envelope JSON, all eight JSON receiver types, signed-receipt authentication, producer-to-replay/append-head transitions, and signing-domain text. Accepted signed documents are verified against independently fixed event and authority facts. Arbitrary selector bytes now always reach an actual JSON decoder. Receiver fuzzing proves Validate, exact typed round trips and canonical bytes, with package and JSON identities preserved on refusal.

Proofledger has no classification result or producer-to-classifier split; it constructs, binds and verifies a chain. No synthetic 50-row classification matrix is claimed. Its Appender/Reader/Iterator types are interfaces. The in-memory provider tests are named explicitly as interface demonstrations, not durable integration evidence. Storage, cancellation of real providers, disk manifests, ledgers of test execution, independent acceptance, and human reporting remain owned by the implementations of those interfaces. No provider or consumer source was changed.

## Red states and mutation evidence

The initial new boundary run failed against unchanged production on valid large extents, nonzero genesis refusal and lost overflow identity. A separate red test reproduced the impossible More flag at the maximum sequence after the first fixes. Ten deliberate semantic regressions were then killed: skipped event-hash comparison, predecessor comparison, cursor advance, page-next comparison, native-end check, truncation check, receipt field binding, receiver preservation, short-write refusal and signing-domain validity. All mutations were restored. The mutation records distinguish real failing tests from compilation failure.

Retained exploratory failures include two runs with an unused bytes import after retiring the old quota test, Witness's mutable function-array finding, and strict Errcheck findings for ignored fixture-constructor errors. These were corrected; none was waived or folded into a passing attempt.

## Validation and measurements

Uncached package tests and two race/shuffle repetitions passed. Statement coverage is 84.1%; unreachable defensive branches were not forged to inflate it. Go vet, Staticcheck, strict Errcheck, Witness lint, go fix -diff and production complexity at most ten passed; the whole module builds. Global tests/gates were not rerun, and their previously recorded baseline failures are not represented as resolved. Native execution was Linux/amd64. Cross-platform runtime testing and independent acceptance remain unclaimed.

Each benchmark phase ran once with 30 seconds configured per workload, aggregate CPU/memory profiles and a retained matching binary. The operation counts, absolute observations and phase limitations appear in before_benchmarks.md and after_benchmarks.md. Removed quota-only serializations reduce observed allocations; a single before/after observation is not a statistical performance distribution.

| Workload | Before ns/op | After ns/op | Before/after B/op | Before/after allocs/op |
|---|---:|---:|---:|---:|
| BenchmarkProofLedgerEventHash | 43026 | 28471 | 5943/3716 | 104/66 |
| BenchmarkProofLedgerReceiptVerification | 218069 | 177583 | 11213/6758 | 193/117 |
| BenchmarkProofLedgerStreamingChainReplayPerEvent | 87615 | 42982 | 12254/5574 | 213/99 |

Evidence is retained at /work/engineering-evidence/primitive/proofledger-upgrade-20260910. Every recorded command has a unique directory with source revision/status/hashes, complete argv, relevant environment, toolchain, stdout/stderr, elapsed duration, exit/signal facts and source-stability check. Test JSON retains each top-level unit, subtest, fuzz seed and skip. The final manifest seals retained records, logs, source snapshots, profiles and binaries; scratch files are excluded explicitly. These are local review facts, not a receipt issued by an independent acceptance authority.

## Final fuzz campaigns

| Target phase | Configured seconds | Process seconds | Last reported executions | Exit |
|---|---:|---:|---:|---:|
| fuzz-domain | 30 | 30.656 | 1384399 | 0 |
| fuzz-envelope | 30 | 37.935 | 974929 | 0 |
| fuzz-json-doors | 30 | 31.559 | 484516 | 0 |
| fuzz-receipt-auth | 30 | 31.547 | 233524 | 0 |
| fuzz-replay | 30 | 30.603 | 263435 | 0 |

Targets ran serially, once each, with four workers and the existing fuzz cache. Reported execution counters are retained observations, not comparable throughput or an exploration-coverage claim. Darwin/arm64 and Windows/amd64 test binaries compiled; they were not executed on those operating systems.
