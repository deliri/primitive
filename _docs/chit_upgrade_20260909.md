# Chit upgrade — reviewed 2026-09-09

**Reviewed**: 2026-09-09
**Mode**: local uncommitted package review (`chit`, not `chitauth`)
**Host**: `d@192.168.1.81:/home/d/code/primitive`
**Base**: `8eedc412d3edf3b0490050407df1fe451ae34eef` (v2026.1.26)
**Decision**: APPROVE with comments
**Issue counts**: 0 bugs, 2 suggestions, 0 nits

The structured review below is the verdict. The author's execution notes and
the companion evidence JSON remain attached so the review is not detached from
the Linux race, fuzz, mutation, and benchmark record.

## Summary

The five claimed Chit repairs are implemented correctly in the current working tree. Typed-nil `io.Writer` values are refused by `core.WriterIsNil` before any canonical write; `IssueCatalog` validates the page, copies entries, then lets `attest.Sign` hash that snapshot (Public/Sign therefore cannot change the signed facts); all twelve public scalar `UnmarshalJSON` methods apply Core’s 1 MiB ceiling before scanning; `ManifestSummary` now allows a zero total while still requiring a positive object count and a set digest, matching Receipt’s empty-stream integrity; and entry-name validation uses `strings.SplitSeq` without rewriting component rules. The remaining risk is coverage, not production correctness: the dedicated 1 MiB table does not ratchet the three numeric scalar doors, so those extent checks could be reverted without failing the upgrade tests.

## Issues

### Issue 1 -- Severity: suggestion
- File: chit/boundary_upgrade_test.go:253
- Description: `TestChitJSONScalarExtentTable` ratchets whitespace-padded below/at/above `core.JSONDocumentMaximumBytes` for the nine string-like scalars only (`EntryName`, `ChitID`, `CollectionID`, `Partition`, `SigningDomain`, `CustodyState`, `Cursor`, `ManifestDigest`, `QueryCommitment`). `Version`, `ObjectCount`, and `EntrySequence` all call `validateScalarJSONExtent` in production (`chit.go:51`, `manifest.go:45`, `manifest.go:95`), but they are absent from this table. Those numeric decoders already refuse padded or non-canonical integers after `json.Unmarshal` by comparing `MarshalJSON()` to the raw input, and `FuzzChitExternalJSONDoorInventory` only asserts `ErrJSONContract` for `len(data) > core.JSONDocumentMaximumBytes`. Reverting the three numeric extent checks would still refuse oversized padded numbers (after scanning/allocating) and would still pass both the extent table and that fuzz assertion.
- Suggestion: Drive `Version`, `ObjectCount`, and `EntrySequence` through the same below/at/above padded-input cases (expecting refusal at every size, with the receiver preserved) so the 1 MiB check is ratcheted independently of the canonical-integer oracle.
- Status: open

### Issue 2 -- Severity: suggestion
- File: chit/boundary_upgrade_test.go:273
- Description: `// The compiler-owned inventory constructs each concrete receiver; no parser is duplicated.` restates the type-switch mechanics and embeds design rationale. The switch already exists to bind `chitScalarExtent` to a concrete decoder; the comment does not explain a non-obvious constraint.
- Suggestion: Delete the comment.
- Status: open


---

# Slice notes (author, pre-review)

Base: `8eedc412d3edf3b0490050407df1fe451ae34eef` (`v2026.1.26`). Candidate is uncommitted on `d@192.168.1.81:/home/d/code/primitive`. No version bump, commit, push, or consumer migration has been performed for this slice.

## Production changes

Four defect families were reproduced before correction; failing subtest counts are not counted as separate regressions.

1. **Canonical writer admission:** Payload, QueryPayload, and CatalogPayload accepted typed-nil destinations. They now use Core's existing `WriterIsNil` before calling a writer. The table checks nil pointer/function implementations, exact writes, short/zero/invalid counts, cancellation, deadline, wrapped errors, and errors returned alongside a full count. Invalid bodies must cause no write.
2. **Catalog ownership:** IssueCatalog returned the caller's entry slice and could sign a different value from the one later returned when signer callbacks mutated that slice. It now validates the bounded payload, copies entries before any signer callback, and lets Attest own signing and signer validation. Thirty input-mutation cases cross ten signed facts with mutation during Public, during Sign, and after issuance. Returned documents still authenticate the original payload. VerifiedCatalog's existing input/accessor copy boundary remains covered.
3. **Public scalar byte limits:** nine string-like decoders admitted more than Core's one-MiB document limit through whitespace padding. All twelve scalar decoders now check that shared limit before scanning/allocating. Exact below/at/above cases preserve receivers on rejection; the three numeric decoders also retain canonical positive-integer admission. The outer structured document limits remain 32 KiB, 64 KiB, or Core's page limit as owned by their existing contracts.
4. **Authenticated empty objects:** Receipt already proves empty objects, but Chit rejected a nonempty manifest whose total extent was zero. ManifestSummary now permits zero total bytes while still requiring at least one object and a valid digest. Extent tables check empty prefixes/suffixes, empty-only manifests, signed-size saturation, and overflow refusal with an unchanged digest. Those summaries pass through Issue and Verify.

The measured optimization replaces `strings.Split` in entry-name validation with Go's `strings.SplitSeq`. It removes the temporary component slice and adds no parsing implementation. The zero-allocation ratchet checks maximum component size, component count, and total name bytes with a typed runtime-allocation isolation declaration.

## Ownership and simplicity

Production changes are confined to Chit. No dependency, shared public type, JSON field, signing domain, product policy engine, workflow, transport, filesystem implementation, or clock was added. Chit continues to use Attest, Receipt, Core, Temporal, ID, and Go's JSON/crypto/io/string capabilities. The source inventory test now reads its embedded source instead of the working directory.

Manifest processing retains a digest, counters, and at most one selected entry. Entry-name validation retains no component collection. Catalog copies remain bounded by `core.CatalogPageMaximumEntries`. Work is O(input bytes); auxiliary memory is O(1) for the fold/name scan and bounded for encoded entries/catalog pages. The shared scalar wire ceiling is an admission limit, not a preallocated buffer.

All production functions remain at cyclomatic complexity ten or below. The producer→classifier quota does not apply: Chit authenticates and binds mechanical agreements rather than deriving product classifications. Signature, scope, query, ordering, selection, extent, retention, continuation, and membership boundaries have direct hostile tests.

## Test and fuzz changes

The entire project-local testing protocol was read; its unchanged SHA-256 is `dd83cd7f62c172092546dab6b7c7d5c59753b5e8ae784631a94cff4d6d48126c`. The work adds real red/green proofs, replaces arbitrary count/marker sampling with extent and single-bit boundaries, repairs a reordered JSON fixture that originally kept the same order, replaces duplicate JSON acceptance rows, and removes a silently abandoned invalid-UTF-8 test path.

Catalog admission tests prove that oversized pages made of valid entries, cross-scope entries, duplicate/reversed identities, and wrong/empty continuations are refused before either signer callback. Foreign partition placement is tested at every page position. Three tagged unions exhaust all 256 discriminator bytes with absent/present data. Eighteen JSON types have zero-value and nil-receiver checks. Manifest membership now checks each independent summary field, exact first/interior/last selection, missing selection, refusal-before-consumption, and terminal seal reuse. Numeric fuzzing uses an independent strconv admission oracle; manifest fuzzing checks a separately computed SHA-256 fold, refusal conservation, exact membership, and terminal reuse. Every inventory selector now reaches a real public ingress door. Generic pointer constraints bind decoder methods at compile time.

Nine deliberate mutations failed the tests they were intended to attack: missing nil admission, missing catalog copy, missing scalar cap, rejecting empty-only manifests, reintroducing the component allocation, consuming a rejected sequence, rejecting all numeric inputs, bypassing the catalog entry cap, and accepting a foreign entry scope. Every mutant was restored. The initial mutation runner stopped before mutation six due to an exact source-selector typo; the first five runs were retained and only the last two were resumed. This was a harness failure, not a production defect.

Final Linux race run: **2548 passing test events, 0 failures, 0 skips**, statement coverage **90.2%**. Event counts include parent subtests and fuzz seeds; they are not a count of independent invariants. `go vet`, Staticcheck, errcheck, Witness lint, and production complexity pass. Current test sources compile for Darwin/arm64 and Windows/amd64; native execution was Linux/amd64 only. Full-module gates were not rerun, consistent with the package-by-package scope.

All fifteen fuzz targets completed requested 30-second campaigns with four workers. Two inventory campaigns were rerun after strengthening their selector routing; earlier attempts remain visible. Existing Go build/fuzz caches were retained, and seed-baseline counts and progress are in stdout. No fuzz/test/benchmark child was suspended, terminated, filtered into a claimed full-module pass, or retried silently. The orchestration parent was paused while its active fuzz child finished, allowing the final table edits and test-source synchronization before benchmarking.

| Campaign | Executions reported | Last elapsed report (s) | Exit |
| --- | ---: | ---: | ---: |
| `final-fuzz-JSON-inventory` | 397341 | 31 | 0 |
| `final-fuzz-text-inventory` | 107658 | 31 | 0 |
| `fuzz-FuzzCatalogDocumentJSONSemanticAndAuthorityClosure` | 229312 | 31 | 0 |
| `fuzz-FuzzCatalogPayloadJSONSemanticClosure` | 315034 | 31 | 0 |
| `fuzz-FuzzChitCanonicalWriterResults` | 376049 | 30 | 0 |
| `fuzz-FuzzChitExternalJSONDoorInventory` | 127109 | 31 | 0 |
| `fuzz-FuzzChitExternalTextDoorInventory` | 621655 | 31 | 0 |
| `fuzz-FuzzCursorJSONSemanticClosure` | 1075884 | 30 | 0 |
| `fuzz-FuzzCustodyStateJSONSemanticClosure` | 1080493 | 30 | 0 |
| `fuzz-FuzzEntrySequenceJSONSemanticClosure` | 1119673 | 31 | 0 |
| `fuzz-FuzzManifestStreamingRefusalAndMembership` | 19423 | 30 | 0 |
| `fuzz-FuzzObjectCountJSONSemanticClosure` | 1037237 | 31 | 0 |
| `fuzz-FuzzParseChitIDSemanticClosure` | 1178057 | 30 | 0 |
| `fuzz-FuzzParseCollectionIDSemanticClosure` | 1179186 | 30 | 0 |
| `fuzz-FuzzQueryDocumentJSONSemanticAndSignatureClosure` | 280574 | 31 | 0 |
| `fuzz-FuzzQueryPayloadJSONSemanticClosure` | 1116983 | 31 | 0 |
| `fuzz-FuzzVersionJSONSemanticClosure` | 1104568 | 30 | 0 |

## Before/after benchmarks and profiles

Machine: AMD EPYC 7282, Linux/amd64, Go 1.27.1, GOMAXPROCS=8, GOWORK=off. Other work runs on the server; CPU isolation and power posture were not controlled. Every benchmark requested 30 seconds and ran serially after scoped checks/fuzzing. Matching CPU profiles, memory profiles, and test binaries are retained for both comparison phases.

The final baseline is the released production tree with the **same complete Chit test sources** as the candidate. Equality of every test-source SHA-256 is checked by the evidence generator. An earlier profiled baseline with the initial harness is retained separately; it is not silently substituted into the final comparison. Workloads remain 128/4096 authenticated manifest entries, one/maximum catalog pages, and maximum component size/count validation. Manifest benchmarks verify an independent digest plus exact byte/object totals; catalog benchmarks compare the complete returned payload.

| Benchmark | Before ns/op | After ns/op | B/op before → after | Allocs/op before → after |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkManifestAccumulatorStreaming128-8` | 7,031,135.00 | 6,964,849.00 | 993502 → 989375 | 14233 → 13976 |
| `BenchmarkManifestAccumulatorStreaming4096-8` | 219,809,879.00 | 214,780,738.00 | 31812634 → 31678232 | 459096 → 450881 |
| `BenchmarkVerifyCatalogPageOne-8` | 209,775.00 | 223,987.00 | 15816 → 15817 | 246 → 246 |
| `BenchmarkVerifyCatalogPageMaximum-8` | 4,989,965.00 | 4,904,009.00 | 1177835 → 1178221 | 9879 → 9880 |
| `BenchmarkEntryNameValidate/single-component-8` | 159.20 | 60.90 | 16 → 0 | 1 → 0 |
| `BenchmarkEntryNameValidate/maximum-components-8` | 7,995.00 | 3,568.00 | 4864 → 0 | 1 → 0 |

These are single samples per workload per phase; **no latency trend is claimed**. The initial profile identified `strings.genSplit` allocations; the targeted change uses the standard-library iterator. Combined profiles include all six benchmark workloads, so a combined profile share must not be described as a manifest-only share. Cumulative allocated bytes are not retained heap. The candidate CPU profile places the remaining work in standard-library byte scanning, JSON, Ed25519, and SHA-256. The component-slice allocation is absent from the candidate allocation profile. Per-phase CPU, allocation-space, and in-use-space summaries are preserved for review.

## Evidence and review boundary

Raw evidence: `/home/d/engineering-evidence/primitive/chit-upgrade-20260909`. The small review manifest is `_docs/chit_upgrade_20260909_evidence.json`; profiles, binaries, logs, source snapshots, and fuzz progress remain on the server. The manifest lists every run, exact command, exit, source base/status, artifact SHA-256, and all final Go source hashes. All recorded artifact hashes and content-addressed source blobs were checked before this report was written. Runtime evidence is tied to the recorded dirty source snapshots above the base commit. The last untimed catalog admission table and exhaustive foreign-partition positions were added after the benchmark pair; the production sources, benchmark functions, and fixture helpers did not change. The race suite, analyzers, and cross-compiles were rerun for those final tests. All corresponding source snapshots remain distinct in the manifest; report generation changes documentation only.

Review this Chit slice before publication. There is no claim of exhaustive input-space proof from coverage or finite fuzzing, no live-provider exercise in this pure agreement package, and no claim that the full module or macOS/Windows runtime behavior was retested here.


---

## Author follow-up — review 1944eb47 closed, 2026-09-09

Both review suggestions are addressed. The review verdict and original slice notes above are preserved verbatim; their prefix SHA-256 is `04aa2de2486b9a6aeed65a1832924ab0f89fafc57c41acd1551af93c0f8e39a2`.

1. `TestChitJSONScalarExtentTable` now covers all twelve scalar decoders. Version, ObjectCount, and EntrySequence receive whitespace-padded input at Core's byte ceiling minus one, exactly at the ceiling, and plus one. All nine numeric cases require typed JSON/Chit refusal and unchanged receivers.
2. Padding refusal alone cannot distinguish early byte admission from later canonicality refusal. `TestChitNumericExtentPrecedesParsingTable` therefore exercises numeric overflow at the same three extents through each numeric decoder. Below and at the ceiling, the wrapped Go `json.SemanticError` must remain visible through `errors.As`; above the ceiling, Chit must refuse before that parser failure. Every case preserves the receiver and both Core error identities. No timing threshold or custom parser was added.
3. The redundant inventory comment was removed.

Removing the early extent guard independently from Version, ObjectCount, and EntrySequence caused the corresponding oversized row to fail in all three recorded mutation runs. Every production mutation was restored byte-for-byte. These proofs raise the slice's deliberate mutation count from nine to twelve; they are not additional production bug claims.

The final unfiltered Chit race command was `GOWORK=off GOMAXPROCS=8 go test -json -race -count=1 -parallel=8 ./chit` on Linux/amd64, Go 1.27.1. It passed with **2,564 passing test events, zero failures, zero skips**. Counts include parent tests and fuzz seeds. Package-scoped `go vet`, Staticcheck, errcheck, and Witness lint also passed. Build caches were retained; `-count=1` bypassed test-result caching. The focused admission run and all three expected-failing mutation runs are retained separately rather than folded into the race result.

Only `chit/boundary_upgrade_test.go` changed among Go sources since the original final admission checks. Production, benchmark functions, and benchmark fixture helpers remain byte-identical. Timed fuzzing, profiled benchmarks, cross-compilation, and full-module gates were not repeated for this test/comment follow-up. Their original evidence remains bound to its original source snapshots; the earlier 90.2% coverage measurement is historical, not a new coverage run. CPU/memory profiles and matching binaries remain in the original evidence directory.

The companion `_docs/chit_upgrade_20260909_review_followup.json` records the exact source hashes, complete commands, environment, outcomes, source stability, and raw artifact locations for all nine follow-up runs. All 36 referenced run artifacts were hash-verified. Source remains uncommitted; this follow-up performs no version bump or push.
