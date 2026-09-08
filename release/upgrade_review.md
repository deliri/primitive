# Release upgrade review — September 8, 2026

Status: reviewed and approved by the user for v2026.1.24 publication. The starting revision is Keygen's published v2026.1.23,
`afcab259661321720ed0fb9d8744412c71e307be`.

The complete 2,214-line `_docs/testing_protocol.md` was read before test edits;
its SHA-256 remains `dd83cd7f62c172092546dab6b7c7d5c59753b5e8ae784631a94cff4d6d48126c`.
All 63 original package Go files are preserved. The historical baseline manifest
remains unchanged; the evolving [upgrade manifest](upgrade_evidence.json) records
commands and exact source snapshots for subsequent checks.

## Regression-proven production changes

- Module checksums reject noncanonical Base64 pad bits and ignored CR/LF instead
  of silently rewriting their identity. Decoding uses Go's strict Base64 decoder
  into fixed storage, removing its per-checksum temporary allocation.
- A dependency stream at the exact 65,536-package ceiling can reach EOF. An
  additional record, including a truncated record, still refuses all partial facts.
- Published dependency modules and build-provenance selector lists must be explicitly present. Missing/null collections no longer become empty admitted facts. Six refusal leaves failed before the two owning wire guards were added.
- Invalid material requests return zero intent alongside their typed error.
- Signing-seed JSON enforces the existing 64 KiB document limit before decoding.
- Dependency observation refuses a working directory outside the verified root.
- Dependency observation refuses a build plan naming another commit.
- Build-tool parsing and hashing are bounded to the captured file extent, with a
  one-byte growth probe. Short input preserves `io.ErrUnexpectedEOF`; both growth
  and shrinkage refuse all build-info/digest output.

The original admission tests failed on seven dependency leaves and five material
leaves; these are four defects, not twelve independently counted defects. The
repository-binding table exhausts the four combinations of root/commit equality:
the matching combination passed before the fix and all three substitutions failed
the intended refusal verdict. Two direct production comparisons now close those
independent binding gaps. Both tool and plan validators already pin the sole
admitted Go toolchain; a redundant equality rule was unnecessary.

Material response JSON now carries encoded seed text through structural decoding.
Live secret custody is created only after required sibling fields are admitted.
This is structural hardening, not a separately runtime-proven leak. A compiler
witness prevents the wire struct from acquiring live seed custody again. Cleanup
errors are retained, fixed secret buffers cleared, and successful replacement
consumes prior custody only after the replacement is admitted.

The private dependency constructor now takes exclusive ownership of freshly
built module storage. Both production callers retain no mutable alias. This
removes the redundant decoder clone and both observation clones, and uses Go's
`slices.SortFunc` directly. No public constructor or additional wrapper was added.
A public decoder table proves that overwriting source bytes and replacing another
receiver cannot alter retained facts, at empty/singleton/near-ceiling/ceiling sizes.

## Test and fixture changes

Checksum and seed tests preserve both receiver identity and original contents on
refusal. New semantic fuzz oracles compare dependency facts with admitted input,
and opened signing keys with Go's Ed25519 derivation of the input seed. Deliberate
checksum and seed mutations were rejected. A stable, idempotent seed substitution
passes round trips but fails the independent derivation oracle, proving that the
oracle checks more than serialization consistency.

The dependency live test previously observed this checkout while supplying proof
from an unrelated temporary repository. It now creates, commits, verifies and
observes one isolated minimal module. Go supplies the pinned cached x/sys version
and checksums; the test compares exact output facts across all canonical targets.
Network module lookup is disabled. Shared fixture Git execution uses Process;
fixture writes and hook-directory preparation use Filestore; ambient observations
use Hostfacts; wait durations use Temporal. TempDir ownership is visible at the
updated test call sites. The later fixture slice migrates the remaining direct file/process/environment/time effects through Filestore, Process, Hostfacts and Temporal. Native handle operations remain recognizable Go calls after Filestore acquisition.

The Go-version grammar table had masked rows: malformed lines lacked a terminal
newline, and the next-patch case named the current patch. Inputs now reach their
claimed guards; redundant field-count examples were removed. Removing exact version equality
in a mutation run now fails five intended rows.

## Verification so far

The original suite passed 865 leaf tests / 931 test-subtest events under race,
with zero skips and 83.7% statement coverage. The admission slice later passed
898 leaves / 969 events under race, zero skips and 83.8% coverage. Scoped vet,
staticcheck, errcheck, nilaway, go fix -diff, goconst, gocyclo and witness passed.
Deadcode reported dependency APIs unreachable from this test binary, with no
Release-owned findings. Later receiver-oracle changes passed filtered race checks.
These checks retain their exact filters and source versions; they are not silently
attributed to newer edits.

The binding and affected repository/build-process slice passed 80 test/subtest
pass events under race, zero failures/skips. A preceding attempt caught a fixture
bug introduced during migration: an empty payload requested a forbidden zero
ByteCount ceiling. It now uses a positive ceiling while retaining zero payload.
The failed setup remains in the evidence record.

Timed fuzz campaigns ran serially after the admission benchmarks, one worker,
30 seconds each: 64,361 dependency executions and 257,438 material executions,
both passed. Fuzz cache state and exact source are retained. After the ownership change, a
further dependency campaign passed 106,756 executions with one worker / 30 seconds. The later exact-value JSON campaign passed 32,577 executions, and the text campaign passed 202,719, each with one worker and 30 seconds. Those were intermediate campaigns; the final eight-target results are recorded below.
The later ownership slice passed the entire package under race: 976 pass events,
zero failures/skips, 83.9% statement coverage. Scoped vet, staticcheck, errcheck,
nilaway, go fix -diff, goconst, gocyclo and deadcode completed; no Release-owned
deadcode findings. Witness initially rejected a fixture failure message without
concrete commit coordinates; the diagnostic was corrected and Witness passed
before benchmarking. The failure remains recorded. Full-module gates and native
Linux/Windows execution have not been performed. Later Linux amd64/arm64 and
Windows amd64 compile-only checks passed.

The shared JSON fuzzer now requires the JSON decoder at compile time and sends
all selector bytes to real ingress doors. Go jsontext normalizes object order and
string escaping while preserving raw numeric tokens: adjacent generations above
2^53 remain distinguishable. An idempotent generation-substitution mutation was
caught even though its round trip is stable. Full race tests through collection
presence fixes passed 995 test/subtest events with zero failures/skips and 84.0%
coverage; later accessor checks have their own source-bound scope.

## Measured admission slice

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocations | After allocations |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| MaterialResponseDecodeOwnedLifetime | 20,907 | 20,267 | 16,897 | 16,743 | 207 | 203 |
| ReleaseSigningSeedDecodeOwnedLifetime | 343.7 | 285.6 | 256 | 112 | 5 | 2 |
| BuildDependenciesUnmarshalSparse | 3,040 | 3,372 | 1,957 | 1,805 | 44 | 41 |
| BuildDependenciesUnmarshalMaximum | 1,195,153 | 1,241,509 | 485,339 | 362,053 | 8,259 | 7,230 |

All comparisons use frozen matching harnesses, serial requested-30-second runs,
explicit CPU/memory profiles and matching binaries. Material baselines were
reconstructed from preserved original production after fixes; they were not
captured chronologically before editing. A first harness attempt failed when it
tried to destroy an unset seed; that failure and the first response sample remain
historical diagnostics, excluded from the corrected v2 comparisons.

Dependency memory use decreased while both dependency timing samples were slower.
These are single samples, not statistical trends. The seed's after allocation
profile attributes almost all allocation to owned secret storage and JSON token
decoding; the temporary Base64 allocation is gone. The intermediate admission profile identified a second module slice using about
16.5% of allocations. The ownership after profile no longer contains that clone.
Its earlier after samples (1,909 / 436,000 B/op) remain recorded historically;
the table above uses the latest ownership samples. The sparse CPU profiles differ
substantially in Go runtime/OS activity; these samples do not isolate a timing
regression or establish a speedup. The final measurement section below adds the remaining workloads; historical samples retain their own source coordinates.
Raw profiles and binaries are ignored locally; reports bind their hashes.

## Build-tool extent evidence

The existing file-size validator was correct, but its result did not bound the
subsequent hash read. A behavior-preserving extraction exposed that exact handoff
for deterministic tests: the captured extent equal to the real compiler passed,
while both adjacent extent mismatches incorrectly succeeded before the fix.
This models the after-Stat handoff directly; it does not claim to have scheduled
an actual concurrent writer. The corrected helper uses Go SectionReader for both
`debug/buildinfo.Read` and hashing, then requires the exact observed byte count.
No alternate executable parser, reader engine or mutable runtime was introduced.

The whole package then passed race detection with 980 pass events, no failures or
skips and 84.0% coverage. Nilaway found that the extracted caller could not see the
FileInfo nil guard inside validation. Validation now returns the admitted extent
directly, and the subsequent focused race run (19 pass events), nilaway and
complexity check passed. Other scoped analyzers passed on the preceding version;
all exact source versions and the failed nilaway attempt remain recorded.

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocations | After allocations |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| InspectBuildToolExecutable | 11,498,329 | 11,530,403 | 3,114,916 | 3,082,244 | 41,589 | 41,590 |

Both requested-30-second runs inspected the identical 16,071,392-byte installed
Go executable, SHA-256 `132b69336a1f809932a8a20b0201dbbb980e86e3a323ae32e893639d83d71598`.
The frozen harness independently hashes it through Go SHA-256 before timing and
checks the exact digest and compiler identity for every timed operation. The
before capture preceded any edit to build_tools.go; that file was checked against
the preserved original. CPU/memory profiles, matching binaries, and a digest-
verified copy of the compiler fixture are retained locally. The small timing
difference is one pair of samples, not an established performance regression or
speedup. This closes bounded extent hashing, not every file-observation question.

## Dependency access

Both public accessors previously called full collection validation. A normal
indexed walk repeatedly validated every path/version/checksum, making the walk
quadratic in the module count. Construction and publication still validate every
fact; Count and At now read sealed, exclusively owned private storage with zero
and index guards. They expose only value copies. No new cache, index, iterator
runtime or mutable state was added.

The hostile access table checks zero/empty/singleton/ceiling collections, extreme
out-of-range indices, every admitted slot, and returned-value mutation. A deliberate
mutation returning slot zero for all indices failed the exact-fact verdict. The
focused race run passed 31 test/subtest events. Vet, staticcheck, nilaway and Witness
passed before the after benchmarks.

The matched 1,024-module walk measured 330,201,305 ns/op before and 11,252 ns/op
after, including exact equality checks for every visited module. Before recorded
1,194 B/op and 0 allocs/op; after recorded 0 B/op and 0 allocs/op. The CPU profile
attributes 95.88% to full validation before; after, that rescan is absent and the
benchmark's equality checks dominate. Allocation profiles include untimed fixture,
Go runtime and profiler setup; the earlier byte counter is retained as reported,
not described as a removed per-walk application allocation.

The direct singleton pair (765.1 versus 11.31 ns/op) is retained as diagnostic:
the after run hit Go's one-billion-iteration cap after 11.31 effective seconds.
A new frozen comparison batches 16 singleton walks per operation, with the old
accessors reconstructed from the preserved pre-edit source. The paired samples measured 13,374 ns per 16-walk batch before and 182.9 ns per
16-walk batch after (835.875 and 11.43125 ns/walk when normalized). Both report
0 B/op and 0 allocs/op; effective timed durations exceeded 35 seconds. All runs request 30 seconds
and retain actual iteration counts, CPU/memory profiles and matching binaries.
These are individual paired samples, not performance distributions.

## Material custody, inventory and fixture ratchets

The old material null-seed row also injected an unknown sibling field, masking
which guard rejected it. The replacement table builds hostile fields from the
production wire type, owns separate custody per parallel row, and checks exact
remaining bytes as well as handle identity. Equivalent and changed-seed successful
replacement both prove prior custody is destroyed. A mutation destroying custody
on structural refusal failed four intended leaves despite unchanged receiver
fields and unchanged error identity. This is test hardening over the already
correct production refusal path, not another production-defect claim.

The struct inventory now sees defined projections, generic instantiations, local
carriers and anonymous carriers. Its initial run found seven unclassified shapes.
Existing local/anonymous shapes now have package-level names and wire roles;
encoding field order/tags and required-field pointer decoding are unchanged.
Wire carriers have a distinct inventory role from internal flow carriers. Source
scans use Go embed.FS rather than runtime filesystem reads. The same matchers run
against hostile synthetic source, including receiver forms and cyclic source.

The whole package through custody/inventory changes passed race detection with
1,027 test/subtest events, zero failures/skips, and 84.2% coverage. Vet,
staticcheck, errcheck, nilaway, Witness and production complexity checks passed.
The later full fixture-migration run passed 1,028 events under race, with zero
failures/skips and 84.2% coverage. Its scoped vet, staticcheck, errcheck, nilaway,
Witness and complexity checks also passed. Each result retains its own source bytes.

Executable fixture compilation now uses Process with bounded separate output,
Temporal owns its two-minute timeout and wait delay, and Hostfacts supplies its
explicit environment. Independent fixture hashing streams through Filestore and
Go SHA-256/CRC32C. A truncated-tool fixture reads only its 4 KiB prefix. The file
fuzzer acquires its oracle handle through Filestore before removing permissions,
then checks both path standing and held-file bytes without repairing permissions.
Sparse extent setup, file mode changes, repository files/directories and timestamp
restoration now use Filestore acquisition/capabilities and caller-owned native
handles. The weakened-Git-stat row writes in place and restores observed mtime;
it no longer changes inode identity through an atomic replacement. Those repository
cases passed under race. Expected setup/compile failures remain recorded; they are
not counted as production regressions.

## Exact Go streams and cooperative file cancellation

The stream table now checks complete independently declared path/version/checksum
facts on every accepted row and clears a reused observation on every refusal.
The union table decodes each target through the real producer, checks immutable
right-hand input, and compares all published facts through Go JSON. Fifteen earned
rows cover empty targets, duplicates, overlap, ordering, root/version/checksum
conflicts (including a conflict after a successful insertion), and the module
ceiling. This is a fact collector, not a producer/classifier split. Failed private
accumulators are discarded by the public observation path; the test no longer
claims transactional preservation of private scratch. The corrected union and
publication slice passed 31 test/subtest events under race.

Two additional admission gaps were regression-proven: a nested Go module Error
was ignored, and Main could erase supplied version/checksum facts. Four refused
rows failed before the two guards were added. The raw evidence retains the exact
Go 1.27.1 source defining these fields and main-module construction. The new
Go-package-stream fuzz oracle independently reads actual input fields through Go,
compares complete module facts, and refuses hidden conflicts or fabricated facts.
A deliberate checksum substitution failed both exact-fact and conflict checks.
Its seed suite and final 30-second campaign passed; exact counts are recorded below.

Build-tool read-phase cancellation returned usable proof in three synchronized
real-file cases before the fix. Parsing and hashing now check the real context
between bounded standard-library operations. Public VerifyBuildTools already ran
its later version probe with context; this is a read-phase defect, not three
independent public verification leaks. Both file inspection paths observe held
path standing after reading and discard output on refusal. The standing table
covers unchanged, removed, same-sized replacement and canceled observations.
The extent table now proves that admitted sizes reach the format boundary, so a
reject-every-file implementation cannot satisfy all rows. The focused extent and
standing run passed 12 test/subtest events under race.

Regular-file cancellation is cooperative between bounded phases; it cannot
interrupt an OS read already in progress. Callers own files against concurrent
writers. Path and extent observations do not establish an atomic snapshot, and
same-size content mutation with restored metadata is not claimed detectable.
Hashing/stamp scanning use fixed buffers; Go executable parsers allocate metadata
within the admitted file bound. The public documentation now states these limits.

Material Open now has independent per-row custody and compares the signing public
key with Go's derivation from the actual seed. Invalid request/server facts and
destroyed custody cannot expose material; every terminal path consumes the
response and invalidates copied seed handles, and a second Open refuses. Cleanup
errors in the touched material fixtures are checked. Compiler/setup failures are
retained as failed attempts rather than called production regressions.

## Final scoped validation and review limits

The post-review whole-package run passed **1,193 test/subtest events**, zero failures
or skips, with race detection and **84.8% statement coverage**. Coverage is a
measurement, not proof of complete semantic coverage. Scoped vet, staticcheck,
errcheck, nilaway, Witness, production gocyclo <= 10, go fix -diff and goconst
(minimum 4 characters / 3 occurrences / no tests) passed. go fix produced no diff.
Deadcode completed with no Release-owned findings; unreachable dependency APIs
are retained in the raw report. Linux amd64/arm64 and Windows amd64 test binaries
compiled successfully. Those binaries were not executed on native hosts.
Full-module gates remain deferred.

One earlier candidate race run is explicitly **incomplete as source-bound
evidence**: a separate shared-workspace edit changed `id/request.go` during that
command. Its raw passing events remain historical, and the final package run
replaces it as current verification. No Id changes belong to this Release slice.
One accidental `stringer -h` invocation remains a diagnostic record; it is not
counted as a complexity check. The real gocyclo command is separately recorded.

The selection source checks now share one narrow Go AST matcher with ten hostile
synthetic cases. Missing/nonstruct requests, methods masquerading as package
functions, calls in unrelated functions, grouped/embedded fields and parenthesized
calls cannot silently alter discovery. Text parser inventory is derived from the
public Parse functions rather than a second hardcoded list. These tests prove
syntax; existing behavioral selection tables prove resulting identities. The
weaker duplicate material JSON-replacement test was removed after the exact
response and seed tables covered its custody claims.

No new runtime, product policy, state machine, dependency or public API was added.
No Release commit, version bump or push has been performed. The version remains
v2026.1.23 until the user reviews and approves this slice.

## Producer-to-classifier handoff proof

The final audit replaced the scattered selection/preparation examples with an
exhaustive matrix over the closed decision domain: three freshness states ×
three version relations × two clock states × four target slots = 72 rows. It
checks the actual AssessLatest producer, exact effective/signed time facts,
exclusive selection arms, zero inactive evidence, complete candidate summaries,
prepared manifests/artifact/latest/time/assessment, input preservation and replay
idempotence. This exhausts that finite decision domain instead of padding a
50-case quota with repeated invalid inputs. Separate typed-refusal, absence,
immutable-conflict and adjacent-time tables cover admission and boundary seams.

Preparation cannot promote a refused clock observation to ready or refresh
success. Corrected durable time crosses expiration before authority is returned.
Equal-version manifests with changed local or other-target bytes refuse as
immutable conflicts. Advancement now exhausts the nine generation/version order
combinations using real signed and verified producer facts, plus missing proofs,
foreign-stream precedence, isolated ±1 signed timeline boundaries and the existing
signer-rotation checks. Each matrix row has one typed primary class.

A deliberate mutation forced both selection and preparation to classify every
assessment as current while leaving the producer untouched. It failed **52 rows**
(54 events including parents). Correct production passed the new matrices without
production changes. The weaker selection/preparation examples and their helper
were retired after their unique summary, target and timing claims were covered.
Those handoff changes affected tests only and matched the benchmark and
eight-target fuzz production captures at that checkpoint. The later Grok review
changes build-tool inspection as recorded below.

## Final timed fuzz campaigns

The eight latest retained campaigns passed before the Grok review, serially
with one worker and a requested 30 seconds each. They were not rerun for the
build-tool path fix; their decoder and artifact production paths are unchanged.
The manifest retains actual elapsed time, initial/cache corpus state, minimization
budget, source bytes and all outputs. Counts include seed/cache executions; they
are not an exhaustive-input claim.

| Target | Executions |
| --- | ---: |
| FuzzBuildControlledEnvironmentNameSemanticClosure | 328,899 |
| FuzzBuildDependenciesExactFacts | 116,388 |
| FuzzGoPackageStreamExactFacts | 232,387 |
| FuzzInspectBuiltArtifactFileSemanticClosure | 479 |
| FuzzLatestDocumentJSON | 199,871 |
| FuzzReleaseExternalJSONDoorInventory | 26,600 |
| FuzzReleaseExternalTextDoorInventory | 291,578 |
| FuzzReleaseMaterialResponseExternalSemanticOracle | 222,956 |

No failure was found in these campaigns. Earlier deliberate mutation failures,
setup failures, and prior campaigns remain separate records.

## Final measured comparisons

All runs below requested 30 seconds, ran serially with explicit CPU/memory
profiles, and retained matching binaries. Actual durations and every attempt are
in the manifest. Material and indexed-walk results above remain source-bound
historical measurements of unchanged production paths. The source bindings retain the later test-only inventory/lifecycle cleanup.
The subsequent Grok review refreshes the affected compiler-inspection sample;
other measurements remain historical evidence of unchanged production paths.

The native artifact fixtures are retained locally: 3,802,178 bytes with SHA-256
`b4888c29ae81185a30653dfd228ab5a15e093250eddcca12ac413048f629ca14`, and
14,287,938 bytes with SHA-256
`ee20873ab97db64af710dec779888f4ad861689574be3b208fcc4819395e8c2f`.
Both sides read those identical bytes through a frozen benchmark that independently
hashes the input and checks complete artifact equality on every operation. The
before side reconstructs original artifact inspection; it is not the original
chronological sample, whose temporary executable was not retained. One-iteration
fixture capture runs are excluded from comparison samples. The three original
non-file benchmark/fixture source files were byte-for-byte unchanged.

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| VerifyLatest | 1,221,227 | 1,232,481 | 626,600 | 626,576 | 11,538 | 11,537 |
| AssessLatest | 568 | 560.6 | 0 | 0 | 0 | 0 |
| LatestDocumentJSON | 434,115 | 496,025 | 266,248 | 266,001 | 4,534 | 4,533 |
| Dependency decode: one module | 3,040 | 3,178 | 1,957 | 1,805 | 44 | 41 |
| Dependency decode: 1,024 modules | 1,195,153 | 1,307,867 | 485,339 | 361,990 | 8,259 | 7,230 |
| Inspect Go compiler | 11,498,329 | 11,268,388 | 3,114,916 | 3,083,121 | 41,589 | 41,596 |
| Inspect retained 3.8 MB artifact | 3,037,755 | 3,062,783 | 173,044 | 173,593 | 551 | 554 |
| Inspect retained 14.3 MB artifact | 10,387,514 | 11,218,596 | 173,065 | 173,601 | 551 | 554 |

The measurements are mixed. Several timing samples increased; none establishes a
statistical trend or a blanket speedup. The final artifact CPU profiles attribute
96.26% and 94.81% to OS syscalls. The added standing observation costs three
allocations; allocation remains approximately 174 KB across both file sizes.
The refreshed Go-compiler allocation profile attributes about 97.5% to Go build-info parsing.
The profiles do not justify replacing Go's executable parsers or adding a cache
that could hide file changes. The separately profiled module walk removes repeated
whole-collection validation; its 1,024-module sample changed from 330,201,305 to
11,252 ns/op while preserving exact facts and sealed ownership.

## Recorded baseline

| Workload | ns/op | B/op | allocs/op | Timed seconds |
| --- | ---: | ---: | ---: | ---: |
| VerifyLatest | 1,221,227 | 626,600 | 11538 | 36.97 |
| AssessLatest | 568 | 0 | 0 | 36.13 |
| LatestDocumentJSON | 434,115 | 266,248 | 4534 | 37.15 |
| BuildDependenciesUnmarshalSparse | 3,040 | 1,957 | 44 | 33.98 |
| BuildDependenciesUnmarshalMaximum | 1,195,153 | 485,339 | 8259 | 36.30 |
| InspectBuiltArtifactRealExecutable | 2,931,751 | 175,058 | 597 | 35.91 |
| InspectBuiltArtifactRealExecutablePlusTenMiB | 9,835,293 | 175,097 | 597 | 36.13 |

All seven runs requested 30 seconds and passed their workload checks. CPU and
memory profiles and matching binaries are retained for every run. Timed seconds
are iteration count multiplied by reported ns/op; command elapsed time includes
compilation, setup and profiling. All runs were serial on Go 1.27.1, Darwin/arm64,
Apple M1 Max with GOWORK=off, AC power and the same concurrency. These are single
samples, not distributions. The [baseline manifest](baseline_evidence.json) binds
commands, source snapshots, outputs and artifacts, including profile summaries.

## Changed production files

- [artifact.go](artifact.go)
- [artifact_inspection.go](artifact_inspection.go)
- [build_dependencies.go](build_dependencies.go)
- [build_tools.go](build_tools.go)
- [dependency_observation.go](dependency_observation.go)
- [latest.go](latest.go)
- [manifest.go](manifest.go)
- [material.go](material.go)
- [publication_metadata.go](publication_metadata.go)

Changes to tests, fuzz targets, benchmarks and fixture helpers are in the same
Release directory. Reports and package-priority status are the other tracked
changes. About 1 GB of raw local evidence is ignored; only its reports and the
ignore rule are intended for a reviewed commit. No consumer migration is required
by a public API change in this slice.

## Grok review c56d8b74 — both findings addressed

The reported confined-link failure was confirmed. Process returns the link path;
Filestore OpenRead follows its confined target, while HeldStanding intentionally
compares the final directory entry itself. Release had composed those correct
local contracts incorrectly.

VerifyBuildTools now calls Filestore.Canonicalize before inspection and uses the
resolved absolute path for inspection, the Go version probe, and the returned
VerifiedBuildTools. The caller's request remains unchanged. This also intentionally
admits an explicit absolute link to a compiler outside the link's parent directory:
this public request specifies an absolute executable, not a confinement root.
The resolved target is still subject to all executable identity, extent and probe
checks. No Process or Filestore contract was weakened or reimplemented.

The second finding is also addressed. Held-file inspection checks standing before
Stat/build-info parsing/hashing; the existing post-read standing check and joined
cleanup errors remain. No snapshot, path reservation or interruption of an
in-flight native read is claimed.

Seven public table rows cover direct files, confined links, two-hop links,
absolute external targets, missing targets, cycles and cancellation. They drive
real Process runnability and VerifyBuildTools, compare the independently hashed
compiler bytes and exact resolved path, require zero proofs on refusal, and prove
source bytes remain unchanged. Five direct held-file rows pressure unchanged,
absent, foreign, final-link and canceled/closed-handle standing. Removing the
pre-read guard deliberately fails four rows; the real compiler control passes.
The isolated pre-fix symlink run failed exactly two leaves. Two earlier fixture
attempts and one compile failure remain in the evidence and are not counted as
production defects.

The updated complete Release race run passed 1,193 test/subtest events, zero
failures/skips, with 84.8% statement coverage. All scoped analyzers passed again;
Linux amd64/arm64 and Windows amd64 test binaries compiled, without native execution.
The eight earlier timed fuzz campaigns retain their original source coordinates.
No full-module gate, version bump, commit or push was performed.

The affected inspection benchmark was rerun after checks, serially for a requested
30 seconds, with explicit CPU/memory profiles and the matching binary. The exact
16,071,392-byte compiler and benchmark source are unchanged. Original / pre-review /
post-review samples are 11,498,329 / 14,025,243 / 11,268,388 ns/op, respectively;
3,114,916 / 3,082,707 / 3,083,121 B/op; and 41,589 / 41,593 / 41,596 allocations.
The new sample completed 3,250 iterations (36.62 timed seconds). The pre-read
observation adds three allocations against the preceding implementation. CPU
still concentrates in native syscalls, and about 97.5% of allocated bytes belong
to Go build-info parsing. Timing variation is not a demonstrated speedup.
This benchmark measures inspection, not the complete VerifyBuildTools process
probe or its newly explicit canonicalization step. All attempts remain retained.

## Publication approval

The user reviewed this slice and explicitly instructed: "bump, commit, push move on".
The approved checkpoint is v2026.1.24. Earlier uncommitted/version statements above
record the state when those checks ran; their exact source coordinates remain
historical evidence. Objectstore is next. Consumer dependency pins are unchanged.
