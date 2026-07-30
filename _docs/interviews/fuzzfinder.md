# Fuzzfinder package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `fuzz` and its
cleanly renamed Primitive 2026 replacement, `fuzzfinder`. The user selected the
new name because the package finds Go-generated fuzz artifacts; it does not run
fuzz tests. No `fuzz` compatibility package or alias is admitted.
The archive is evidence, not authority. No archived production or test source
was copied.

The consumer demand is real:

- Witness uses all three archive capabilities in production: fuzz artifact
  identity, bounded Go-cache corpus selection, and logical-line counting;
- Peachfuzz uses the same three capabilities in production while adding
  content-addressed custody, explicit sidecar states, durable artifact indexes,
  archive traversal, and reproducer harvesting;
- Kernel has the package available at both its committed and dirty Primitive
  frontiers but has no production import; and
- Bug's committed Primitive pin predates the package and Bug has no production
  import.

The archive is not admissible unchanged. Its specification classifies the
package as test support and says production packages must not import it, while
Witness and Peachfuzz each have production imports. Its corpus-name type does
not represent one truthful Go contract: Go 1.26.5-generated cache and crasher
files use exactly sixteen lowercase hexadecimal characters, while manually
authored seed-corpus filenames are not required to be hexadecimal; the archive
instead accepts any lowercase hexadecimal name from eight through sixty-four
bytes. Invalid regular files are silently treated as neutral, so a Go
toolchain format change can be reported as an empty, complete capture.

The archive also:

- embeds a retention policy while claiming to own no campaign policy;
- returns arbitrary directory errors without a stable Fuzz read identity;
- exposes JSON decoders that allocate before applying the nominal value bound;
- provides a `Write` method that is not an `io.Writer`, has no nil-receiver or
  overflow contract, and can silently wrap its line count;
- combines generic sidecar line accounting with Go fuzz-cache mechanics;
- has no real-filesystem package test, fuzz target, benchmark, layer-triad
  declaration, or complete data-flow inventory;
- duplicates protocol literals in tests; and
- has already drifted incompatibly from the APIs pinned by both production
  consumers.

The 2026 package should therefore be rebuilt as a production primitive, not
copied as test support and not wrapped for compatibility. It should own only:

1. the closed corpus/crasher artifact identity;
2. an exact, explicitly versioned Go-generated fuzz cache/crasher entry-name
   contract;
3. a typed, bounded, deterministic observation of one already-open Go fuzz
   cache directory;
4. explicit retained, ignored, dropped, unsupported-format, and incomplete-read
   accounting; and
5. stable Core-owned error identities for contract, format, and observation
   failures.

It must not own a fuzzer process, campaign scheduling, filesystem mutation,
product evidence schemas, object storage, bundle publication, manifest
verification, finding policy, or generic text-sidecar accounting. Witness and
Peachfuzz keep those responsibilities.

## Evidence boundary


### Source revisions and Primitive pins

| Source | Exact revision or Primitive pin | Archived `fuzz` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`, `Harden capability inventory evidence`); Fuzz tree `857b6d5d6cd0c7defeda1a47a83c95b9bed86607` | Present. Introduced by `2d3c81ff1034a772f7a332acdb85962cbfa09f60` on `2026-07-22T07:27`, `2026-07-22T07:39-04`, `2026-07-22T07:00`; artifact identity generalized by `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` three minutes later. | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; inspected Fuzz and Core files are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` (`2026-07-23T04:01`, `2026-07-23T04:52-04`, `2026-07-23T04:00`) | Committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20` (`2026-07-22T21:25`, `2026-07-22T21:01-04`, `2026-07-22T21:00`), Fuzz tree `283e7cffc331fa569127869b59aa4ef596c5afae`. Dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` (`2026-07-27T00:33`, `2026-07-27T00:47-04`, `2026-07-27T00:00`), the archive-head Fuzz tree. No production source imports either generation. | Broad pre-existing dirty migration. The committed and dirty pins are different evidence. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` (`2026-07-24T15:52`, `2026-07-24T15:58-04`, `2026-07-24T15:00`) | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` (`2026-07-22T07:30`, `2026-07-22T07:53-04`, `2026-07-22T07:00`), Fuzz tree `c8d0c8b2f72be867e0c6d87c91cd160966abd30a`. Eight production files import it. | Only the pre-existing untracked `.ledger_pending.md` was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` (`2026-07-24T15:52`, `2026-07-24T15:54-04`, `2026-07-24T15:00`) | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` (`2026-07-20T10:59`, `2026-07-20T10:21-04`, `2026-07-20T10:00`), where Fuzz is absent. No production source imports it. | Only the pre-existing untracked `.ledger_pending.md` was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` (`2026-07-24T15:52`, `2026-07-24T15:50-04`, `2026-07-24T15:00`) | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` (`2026-07-23T17:51`, `2026-07-23T17:17-04`, `2026-07-23T17:00`), Fuzz tree `283e7cffc331fa569127869b59aa4ef596c5afae`. Six production files import it. | Only the pre-existing modified `.ledger_pending.md` was observed. |

The archive history material to the decision is:

- `2d3c81ff1034a772f7a332acdb85962cbfa09f60`: separate neutral fuzz
  contracts from Peachfuzz;
- `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`: generalize fuzz artifact
  identity;
- `8c20a20138919f725269dd5a4d820bdf7081b77e`: consolidate typed exchange
  and runtime safety;
- `cf8fa882ee56bb263220a1466c6deae08e466018`: establish permanent primitive
  boundaries;
- `d259789e87bcadb829c5ffac72c6c91ccc604098`: centralize constants and close
  capabilities; and
- `40ded9c104a99cbc4b0b672cd7392901b468d1eb`: harden comparative contracts.

The production consumers are pinned to the earlier API, which exports
`FuzzCorpusEntryNameMinBytes`, `FuzzCorpusEntryNameMaxBytes`,
`FuzzCorpusDirectoryReadBatchSize`, `ArtifactIndexMaxEntries`,
`GoFuzzDataDirectoryName`, and `ErrFmtContract`
(`archive@773add8ba0fc1a9453cc06c8558b8541c1fc8ce9:fuzz/corpus.go:15-20`).

Archive HEAD moved these values into Core and renamed several of them:

- `FuzzArtifactIndexMaximumEntries`;
- `FuzzCorpusEntryNameMinimumBytes`;
- `FuzzCorpusEntryNameMaximumBytes`;
- `FuzzCorpusDirectoryReadBatchSize`; and
- `FuzzGoDataDirectoryName`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/fuzz_constants.go:3-12`).

Witness still references `fuzz.GoFuzzDataDirectoryName`
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter.go:624`), while Peachfuzz
still references `foundationfuzz.ArtifactIndexMaxEntries` and
`foundationfuzz.GoFuzzDataDirectoryName`
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/fuzz_evidence.go:41`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/fuzz_evidence.go:86`;
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/location_contracts.go:183`).

Updating either consumer directly to archive HEAD would therefore require a
real source migration. Primitive 2026 must update the real call sites and
delete the old names. It must not add aliases, wrappers, or compatibility
shims.

## Capability ownership

The specification calls Fuzz a set of bounded, product-neutral Go fuzz
evidence helpers. It claims:

- a closed corpus/crasher artifact kind;
- logical-line counting;
- deterministic bounded corpus-name selection from an injected directory
  stream;
- at most a Core-owned number of retained names;
- canonical ordering;
- explicit dropped-entry accounting; and
- no whole-corpus load
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/SPEC.md:3-7`).

It explicitly excludes:

- the fuzzer process;
- filesystem mutation;
- product artifact schemas; and
- campaign policy
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/SPEC.md:9-10`).

The production implementation consists of three independent pieces.

### Artifact identity

`ArtifactKind` is a closed `uint8` enum with only Corpus and Crasher valid.
Its token, validation, parsing, and JSON behavior are compiler-visible and
retain the Core-owned `ErrFuzzContract` identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/artifact_kind.go:11-74`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:10`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/fuzz_constants.go:5-7`).

This is a genuine shared protocol primitive. Witness and Peachfuzz both embed
it in typed persisted evidence rather than converting it to local strings.

### Go corpus-entry observation

`FuzzCorpusEntryName` stores a validated lowercase-hex string.
`FuzzCorpusSelection` retains a sorted, deduplicated prefix of at most 128
names, defensively copies its output, and saturates its dropped counter.
`SelectFuzzCorpusDirectory` reads an injected `fs.ReadDirFile` in fixed
batches, returns partial selection on read failure, treats invalid names and
subdirectories as neutral, and counts valid non-regular entries as dropped
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:15-189`).

Memory is bounded by the fixed retained-entry ceiling rather than corpus size.
The selector does not open, hash, copy, mutate, or close payloads. The caller
owns the supplied directory and every later file operation.

### Logical-line accounting

`FuzzLogicalLineCounter` counts newline bytes and adds one for a final
unterminated fragment. Arbitrary chunk boundaries do not change the result.
`CountFuzzLogicalLines` applies the same rule to a complete byte slice
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/stream.go:6-40`).

The mechanism is useful, but the archive has not established that it belongs
to Fuzz. Witness uses it for general sealed test sidecars, not specifically for
fuzz output
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/test_pending_output.go:266`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/benchmarks.go:1755-1787`).

## Archive evidence

### Archive census and fresh focused proof

At archive HEAD, Fuzz contains:

- 307 production Go lines;
- 521 Go test lines;
- a 10-line specification;
- thirteen named deterministic tests;
- no fuzz target; and
- no benchmark.

A fresh focused run passed:

```text
go test ./fuzz
```

`go vet ./fuzz` also passed. `gocyclo -over 10 fuzz` reported only two test
functions, at complexity 12 and 11; it reported no production function above
10. This proves current compile, deterministic test, vet, and production
complexity coherence for the archive's implemented contract.

It does not prove ownership or contract truth. The focused suite never crosses
the real `os.File` directory path, never runs an actual fuzz callback, and
never measures the claimed bounded-allocation behavior.

A focused Witness consumer rerun passed:

```text
go test ./internal/exec \
  -run '^TestPreserveFuzzCorpusViaRunwriterPrimitiveBoundMatrix$' -count=1
go test ./internal/verify \
  -run '^TestVerifierLayerTriadFuzzEvidenceIndexAccounting$' -count=1
```

A focused Peachfuzz rerun was attempted without changing its dependency state.
It stopped before compilation because the downloaded bytes for its exact
Primitive pseudo-version did not match the existing `go.sum` entry:

```text
github.com/deliri/primitive/v2026@...-3f74d8fc35b4:
checksum mismatch
```

No checksum or pin was bypassed or rewritten. Peachfuzz's production and test
source remains valid interview evidence, but this run did not independently
reproduce its green state. The checksum mismatch is a dependency-provenance
blocker for any later claim that the committed Peachfuzz pin is reproducibly
buildable.

### Archived strengths worth preserving

### 1. Closed typed artifact identity

Corpus and crasher are not raw path suffixes or string conventions. The enum
owns `Validate`, JSON closure, parsing, and stable error identity. Both real
consumers use that type in persisted structures.

This is exactly the compiler-owned shape Primitive 2026 should retain.

### 2. Bounded deterministic selection

The selector retains at most 128 names regardless of directory size, keeps
them in canonical order, suppresses duplicates already retained, returns a
defensive copy, and uses saturating dropped accounting
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:61-118`).

The archive tests exercise:

- below, exact, and one-over limits;
- ascending and descending input;
- adversarial interleaving;
- duplicates;
- hostile internal order;
- saturation;
- partial reads;
- nil entries;
- symlinks; and
- cross-batch order
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus_test.go:107-247`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus_test.go:273-350`).

That is a strong bounded-state algorithmic core.

### 3. Partial observation survives source failure

The selector processes entries returned with a terminal read error before
returning the error. Its test proves that one retained entry survives an
injected non-EOF read failure
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:152-165`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus_test.go:302-329`).

Witness converts that partial result into a durable incomplete index and
scrubs the local cache path. Peachfuzz converts it into an explicit
enumeration-failed index. Preserving partial facts rather than discarding them
is the right evidence shape.

### 4. Receiver stability at JSON rejection

The nominal name and artifact-kind decoders assign only after parse succeeds.
Tests prove nil receivers fail with `errors.Is(..., core.ErrFuzzContract)` and
malformed input leaves a preexisting receiver unchanged
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/artifact_kind_test.go:30-58`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus_test.go:60-105`).

The 2026 design should retain that property wherever a JSON boundary remains.

### 5. Caller-owned filesystem lifetime

`SelectFuzzCorpusDirectory` states that the caller owns and closes the
directory (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:144-147`). Both consumers obey this:
Witness opens and defers close around selection
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter.go:672-686`);
Peachfuzz closes explicitly and records close failure as incomplete evidence
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/fuzz_evidence.go:49-79`).

That is an honest lifetime boundary.

### Archived Primitive dependents

A complete production import search at archive HEAD found zero Primitive
packages importing `foundation/v2026/fuzz`.

Core refers to the package path only as an architecture/governance identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_constants.go:61-72`). That is not a runtime
dependent.

The archive's admission demand therefore comes entirely from external
consumers, not Primitive-internal call sites.

## Consumer evidence

### Kernel

Kernel committed HEAD has no production import of Fuzz. Its committed
Primitive pin contains the earlier package, and its dirty pin contains the
archive-head tree, but neither Kernel frontier consumes it.

Kernel's relevant local capability is different: it inventories Go test
ceremony with an AST scan and counts `Test`, `Fuzz`, and `Benchmark` function
declarations into typed per-package and per-component statistics
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/teststats/main.go:1-5`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/teststats/main.go:32-53`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/teststats/main.go:342-400`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/teststats/main.go:446-528`).

Kernel also has many direct trust-boundary fuzz tests. One strong example
mutates HTTP request bytes and proves rejected input does not partially commit
into the destination
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:telemetry/ingest/ingest_test.go:227-254`). Its ledger fuzz tests prove
canonical validity, deterministic output, round-trip values, typed unsupported
errors, and chain linkage
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/fuzz_test.go:11-179`).

These are useful testing patterns, not production Fuzz package capabilities.
Primitive must not turn test discovery, ceremony statistics, fuzz budgets, or
Kernel's product reporting into Fuzz responsibilities.

One Kernel-local gap is visible in the AST inventory: it classifies functions
only by name prefix and does not validate the Go testing signature
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/teststats/main.go:383-397`). That concern belongs to Kernel's test
inventory owner, not Primitive Fuzz.

### Witness

Witness is the first production admission proof.

Eight production files import Primitive Fuzz:

- `internal/artifact/human.go`;
- `internal/core/run_record.go`;
- `internal/evidence/paths.go`;
- `internal/exec/benchmarks.go`;
- `internal/exec/fuzz.go`;
- `internal/exec/fuzz_preservation_runwriter.go`;
- `internal/run/run.go`; and
- `internal/verify/verify.go`.

#### Production flow

Witness stores `fuzz.ArtifactKind` in both the durable Fuzz evidence index file
and the ledger record
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/run_record.go:1017-1041`).

The run-side producer:

1. derives `$GOCACHE/fuzz/<package>/<target>`;
2. opens the real cache directory;
3. calls `fuzz.SelectFuzzCorpusDirectory`;
4. retains partial selection and dropped accounting;
5. opens and streams selected payloads through the run writer;
6. constructs a typed corpus index;
7. publishes canonical JSON;
8. records the projected index fact; and
9. rolls the just-published index back if fact recording fails
   (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter.go:619-760`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter.go:862-938`).

The persisted index includes exact schema version, kind, files, bytes, count,
complete state, dropped count, and scrubbed error
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/run_record.go:1017-1037`).

The independent verifier:

- finds typed corpus and crasher indexes from canonical suffixes;
- validates each declared payload;
- rejects kind mismatch;
- scans for unindexed fuzz payloads; and
- checks regular-file and byte-size facts
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/verify/verify.go:2650-2805`).

This is the strongest consumer gem: the reusable selector participates in a
real producer -> writer -> persisted index -> manifest -> independent verifier
chain. Primitive should preserve the small neutral primitive, not copy the
Witness pipeline.

#### Witness proof

Witness's real-filesystem bound matrix stages 127, 128, and 129 cache files,
runs the real run writer, decodes the on-disk index, and checks exact retained
and dropped counts
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter_test.go:51-143`).

Additional tests cross real filesystem faults:

- a non-directory cache source creates a durable incomplete, path-scrubbed
  index and no fake payloads
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter_test.go:873-980`);
- nested directories create no false drops
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter_test.go:982-1052`); and
- a broken source entry produces a durable partial index
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter_test.go:1054-1142`).

The verifier declares a named Fuzz evidence layer triad
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/verify/verify_test.go:10022`), and the focused producer and
verifier tests passed during this interview.

#### Witness ownership and gaps

Witness correctly keeps product-owned:

- `FuzzEvidenceIndexFile` and `FuzzEvidenceIndexRecord`;
- bundle-relative paths and stems;
- publication and rollback;
- ledger projection;
- complete/advisory outcome policy;
- operator error scrubbing;
- manifest reconciliation; and
- human rendering.

Those must not move into Primitive Fuzz.

Verified gaps:

1. Witness is a production importer despite the archive's test-support-only
   rule.
2. Witness is pinned to the old Fuzz API and cannot compile unchanged against
   archive HEAD.
3. `sidecarFileLineCount` and `sidecarDataLineCount` use Fuzz for generic
   sealed test-output sidecars, showing that logical-line counting is owned too
   broadly by the archive package
   (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/benchmarks.go:1755-1787`;
   `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/test_pending_output.go:266`).
4. The real-filesystem bound test duplicates raw 127/128/129 values instead of
   deriving the shared ceiling from the compiler-owned contract
   (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter_test.go:83-97`).
5. After selection, Witness calls `os.Stat` and then `os.Open` on a reconstructed
   path. A source name can be replaced between enumeration, stat, and open; a
   symlink to a regular file is followed by `os.Stat`
   (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter.go:739-756`).
   The selector proves a name observation, not stable payload custody.

The fifth gap must be closed at the actual filesystem capability owner. Fuzz
must not claim payload identity merely because it selected a name.

### Bug

Bug has no Primitive Fuzz package at its committed pin and no production
import or local duplicate of the archive's artifact identity, cache selector,
or line counter.

Bug does provide strong examples of the testing protocol's fuzz boundary:

- the hostile bug-document target bounds input, crosses a real temporary-file
  parser path, validates every produced typed issue, rejects silent corruption,
  and reruns the parser to prove determinism
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/fuzz_hostile_bug_doc_test.go:14-100`);
- the candidate self-test target uses strict typed JSON, binds accepted fields
  to the signed release target, and proves canonical round trip
  (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update_fuzz_test.go:11-66`); and
- the offline-license target enforces the compiler-owned byte cap, stable
  `ErrLicenseContract`, zero output on rejection, `Validate()` on success, and
  pinned signature verification
  (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license_fuzz_test.go:18-69`).

These tests are gems for reconstructing Fuzz's own hostile proof. They do not
justify moving Bug parsing, file staging, license verification, or fuzz
campaign policy into Primitive.

### Peachfuzz

Peachfuzz is the second and broadest production admission proof.

Six production files import Primitive Fuzz:

- `internal/app/archive_fuzz_objects.go`;
- `internal/app/cycle.go`;
- `internal/app/fuzz_evidence.go`;
- `internal/core/location_contracts.go`;
- `internal/finding/harvest.go`; and
- `protocol/fuzz_evidence.go`.

#### Production flow

Peachfuzz:

1. opens the target's real Go cache directory;
2. uses Primitive's bounded selection;
3. opens each selected file;
4. requires a regular, nonempty, size-bounded payload;
5. streams it into a content-addressed object store;
6. deduplicates by digest;
7. builds a typed corpus index;
8. stores the index as another immutable object; and
9. returns a typed captured sidecar reference with digest, retained bytes, and
   logical lines
   (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/fuzz_evidence.go:18-164`).

Its protocol owns a fixed four-member Fuzz evidence set: stdout, stderr, corpus
index, and crasher index. It gives sidecars explicit absent, empty, captured,
and lost states, and gives artifact indexes explicit complete, partial, and
enumeration-failed states
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/fuzz_evidence.go:306-378`).

`FuzzArtifactIndex` validates:

- artifact kind and state;
- the entry ceiling;
- exact count;
- complete/dropped consistency;
- strict canonical digest order;
- unique digests;
- positive artifact sizes;
- checked total-byte arithmetic; and
- exact total bytes
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/fuzz_evidence.go:380-470`).

The run record streams direct sidecar and index digests, decodes each bounded
artifact index, then yields the referenced payload digests. It explicitly
materializes only the Primitive-bounded index, not run history
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runrec/record.go:161-205`).

Archive publication batches the yielded immutable objects under separate
entry and byte ceilings
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/archive_fuzz_objects.go:15-57`).

This is the key Peachfuzz gem: content-addressed custody and explicit evidence
state close the gap between observing a filename and retaining durable,
independently addressable bytes. Those remain Peachfuzz protocol
and object-store responsibilities.

#### Crasher harvesting

Peachfuzz also uses `ParseFuzzCorpusEntryName` to distinguish Go-created
crashers from stray files in `testdata/fuzz/<target>`. Valid non-regular
entries fail loudly, empty files are skipped, accepted files are ingested into
the object store, and the worktree corpus is reset
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/finding/harvest.go:66-115`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/finding/harvest.go:171-210`).

Its fuzz test crosses the real disk -> Harvest -> object-store path and proves
that every yielded finding validates, names exactly the bytes written, refers
to a verifiable object, has exact accounting, and leaves a clean corpus
directory
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/finding/harvest_fuzz_test.go:71-145`).

That is stronger than the archive's fake directory reader, but it also exposes
why the archive name is misleading: the same parser is applied to generated
crashers and cache entries, while its public name claims every Go-toolchain
corpus filename.

## Strong mechanics and proof

### Peachfuzz proof

Peachfuzz source includes:

- real object-store corpus capture and duplicate-content proof
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/review_findings_test.go:840-884`);
- 127/128/129 production-path capture tests
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/review_findings_test.go:886-950`);
- missing, empty, invalid-name, zero-byte, one-byte, and symlink source states
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/review_findings_test.go:952-1048`);
- typed index round trip and hostile contradiction tables
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/fuzz_evidence_test.go:100-173`); and
- explicit enumeration-failure proof
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/fuzz_evidence_test.go:175-183`).

The focused rerun was blocked by the exact Primitive pseudo-version checksum
mismatch recorded above, so these citations are source evidence rather than a
new green execution claim.

### Peachfuzz ownership and gaps

Peachfuzz correctly keeps product-owned:

- sidecar kind and state;
- the four-member evidence set;
- content-addressed artifact and index schemas;
- object-store writes and verification;
- stdout/stderr retention and truncation;
- finding identity and occurrence;
- worktree reset and quarantine;
- archive batching; and
- run outcome policy.

Those must not move into Primitive Fuzz.

Verified gaps and duplicates:

1. Peachfuzz is a production importer despite the archive's test-support-only
   rule.
2. Peachfuzz is pinned to the old API and cannot compile unchanged against
   archive HEAD.
3. The protocol comment describes `FuzzArtifactIndex` as a Witness shape inside
   Peachfuzz, direct evidence of copied ownership prose
   (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/fuzz_evidence.go:397-400`).
4. `FindingHarvestMaxEntries` is a separate raw `128` while the artifact index
   uses Primitive's separate `ArtifactIndexMaxEntries`
   (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/path_contracts.go:50-53`;
   `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/fuzz_evidence.go:41`;
   `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/fuzz_evidence.go:447`). If these are the same cross-package
   invariant, one typed Core-owned ceiling must control both. If they are
   distinct policies, their types and names must make that distinction
   compiler-visible.
5. `HarvestResult.Validate()` unconditionally returns nil despite three
   persisted/accounted counters and the surrounding trust-boundary role
   (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/finding/harvest.go:49-62`).
6. The tests deliberately accept both sixty-four-character SHA-256 names and
   sixteen-character names
   (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/review_findings_test.go:846-855`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/review_findings_test.go:919-923`), further
   entrenching the archive's invented 8..64 convention rather than one exact Go
   toolchain contract.
7. Peachfuzz opens a selected reconstructed path and only then stats the open
   file (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/fuzz_evidence.go:98-136`). This is stronger
   than stat-before-open, but a path replacement before open can still redirect
   custody. The common selector must not imply a no-follow or same-object proof
   it does not own.

## Defects and blockers

### 1. The package layer declaration is false

The specification says production packages `MUST NOT` import this test-support
package (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/SPEC.md:9-10`).

Witness has eight production imports and Peachfuzz has six. The package is
part of production evidence capture and persisted schema. The 2026 package
must be classified as a production primitive. A test-support allowlist entry
cannot make a false boundary true.

### 2. `FuzzCorpusEntryName` conflates incompatible name domains

The archive accepts any lowercase hexadecimal string from 8 through 64 bytes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:15-29`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/fuzz_constants.go:9-10`).

None of the six admissible evidence repositories proves that the archive's
8..64 range matches the supported toolchain's generated-cache or authored-seed
filename domains. It is therefore an informal compatibility guess rather than
a verified contract. Primitive 2026 must name the exact domain, bind it to an
explicitly supported format, and attach admissible evidence before
implementation. It must not preserve the permissive range as a compatibility
shim.

### 3. Unknown regular files can become falsely complete empty evidence

The selector silently ignores every invalid name and every subdirectory
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:169-183`). A valid non-regular name is counted as a
drop, but an unrecognized regular filename is neutral.

That means a new Go-generated filename format, a malformed cache artifact, or
an unsupported toolchain generation can disappear from accounting. Peachfuzz
ratchets this behavior as `invalid names and directories are neutral` and
expects an empty sidecar
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/review_findings_test.go:970-978`).

The result is not truthful enough for evidence capture. The reconstructed
result must distinguish at least:

- ignored subdirectory;
- ignored explicitly permitted noise;
- rejected non-regular entry;
- unsupported regular-file name/format;
- retained entry;
- over-limit drop; and
- incomplete directory read.

An unknown regular file beneath the exact target cache directory must not
silently certify complete absence.

### 4. The retention ceiling and selection rule are policy without an owner

The archive claims no campaign policy, but it hardcodes:

- a 128-entry ceiling;
- canonical lowest-name retention; and
- a dropped-count interpretation
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:61-118`;
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/fuzz_constants.go:4`).

This may be a defensible evidence-safety ceiling, but it is still a policy that
changes persisted consumer facts. Primitive 2026 must say which of these it
owns:

- an absolute product-neutral safety maximum;
- a caller-selected typed retention limit bounded by that maximum; or
- a fixed shared evidence protocol limit.

The current generic integer name does not make that decision visible. The
batch size, by contrast, is an implementation detail and does not belong in
Core merely because the package uses it.

### 5. Stable read-error identity is missing

On non-EOF failure, `SelectFuzzCorpusDirectory` returns the source error
directly (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:160-165`). The test ratchets only
`errors.Is(err, injected)` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus_test.go:302-329`).

Callers can recover the source identity, but cannot classify every directory
observation failure through a stable Core-owned Fuzz read identity. The
reconstruction needs a typed observation error that preserves the underlying
cause with `Unwrap`, so callers can use both:

- `errors.Is(err, core.ErrFuzzObservation)`; and
- `errors.Is`/`errors.As` for the underlying filesystem identity.

### 6. JSON boundaries allocate before enforcing nominal bounds

Both `ArtifactKind.UnmarshalJSON` and
`FuzzCorpusEntryName.UnmarshalJSON` pass the complete byte slice to
`json.Unmarshal` before applying token or name constraints
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/artifact_kind.go:49-62`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:45-58`).

An arbitrarily large quoted string can therefore allocate before the parser
rejects its value. The entry-name JSON methods are not used by either
production consumer and should be removed unless persistence demand is
demonstrated. Any retained JSON boundary must reject an over-ceiling wire value
before allocating the decoded string and must leave the receiver unchanged.

### 7. The line counter has an informal streaming protocol

`FuzzLogicalLineCounter.Write`:

- returns no byte count or error and therefore does not implement `io.Writer`;
- dereferences a nil receiver and panics;
- increments without saturation or overflow identity; and
- has no `Validate()` or terminal snapshot type
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/stream.go:10-33`).

`CountFuzzLogicalLines` accepts a whole byte slice, while the production
streaming user has to write its own read loop because the counter cannot be
passed to `io.Copy`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/stream.go:35-40`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/benchmarks.go:1755-1787`).

The mechanic is sound; the contract is not compiler-composable. It should not
remain in Fuzz merely because one consumer uses it for fuzz output. A generic
evidence/text owner may later expose a typed saturating `io.Writer` counter.
If Fuzz retains a scoped counter, its name and API must restrict it to fuzz
output and close nil, overflow, and output validation.

### 8. Injected duplicate over-limit observations overcount drops

Once the selection is full, a new name larger than every retained name
increments `dropped` and is not remembered. Observing that same omitted name
again increments `dropped` again
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:76-96`).

A native directory cannot contain duplicate names, but the public boundary is
an injected `fs.ReadDirFile`; duplicate hostile observations are not forbidden
or tested. The current counter means `over-limit observations`, while the
specification and comments call them dropped entries. The reconstructed type
must define the unit exactly and test duplicate retained and duplicate omitted
names.

### 9. Selection does not establish payload custody

The archive returns names only. It does not hold an entry capability, prevent a
rename/symlink swap, open no-follow, hash content, or verify that the later
opened file is the object enumerated.

Both production consumers reconstruct a path and open later. Their product
pipelines correctly own copying and durable storage, but neither can infer
stable source identity from selection alone. The 2026 API and documentation
must call the output an observation, not captured evidence. Safe descriptor-
relative/no-follow opening belongs to the filesystem capability owner and must
be composed explicitly where hostile local mutation is in scope.

### 10. Validation and documentation closure is incomplete

`FuzzCorpusSelection.Validate` checks bound, entry validity, and strict order,
but does not reject an internally impossible combination such as nonzero
dropped count with fewer than the fixed maximum retained
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:105-118`).

The result type delegates only to that selection and does not independently
validate its non-regular dropped state
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/corpus.go:120-142`).

The fields are private, which limits external construction, but `Validate()`
claims an ownership boundary and should close every representable invariant.

After the Go directory token moved to Core, an orphan comment remains at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/stream.go:3-4` with no declaration beneath it. This is minor, but
it confirms the Core centralization was mechanical rather than a clean
ownership pass.

### Testing-protocol assessment

The project testing protocol was read completely before this interview.
The relevant requirements are:

- evidence paths need positive, negative, and neutral proof at every owned
  layer (`foundation@working-tree:_docs/testing_protocol.md:575-604`);
- owned evidence layers declare top-level tests containing `LayerTriad`
  (`foundation@working-tree:_docs/testing_protocol.md:606-611`);
- serious parsers, writers, and verifiers need hostile tables or exhaustive
  matrices (`foundation@working-tree:_docs/testing_protocol.md:625-638`);
- every production struct in a trust-boundary package needs a classified
  data-flow inventory (`foundation@working-tree:_docs/testing_protocol.md:1046-1084`);
- fuzz targets belong at parsing, enum, and external sidecar boundaries
  (`foundation@working-tree:_docs/testing_protocol.md:1210-1228`);
- every fuzz callback needs a semantic oracle, not merely no panic
  (`foundation@working-tree:_docs/testing_protocol.md:1245-1262`); and
- tiny closed domains should use exhaustive hostile tests rather than consume
  fuzz budget (`foundation@working-tree:_docs/testing_protocol.md:1287-1288`).

### What the archive proves

The archive does prove:

- typed `errors.Is` classification;
- receiver unchanged after decoder rejection;
- positive/negative/neutral name cases;
- lower and upper name boundaries;
- below/exact/above selection limits;
- ascending, descending, interleaved, duplicate, and saturated selection;
- defensive-copy output;
- nil reader and nil entry rejection;
- partial read-error preservation;
- symlink/non-regular accounting;
- cross-batch canonical order;
- logical-line chunk invariance; and
- production `gocyclo <= 10`.

This is materially stronger than a happy-path helper suite.

### What remains unproved

The archive does not prove:

1. an exhaustive 256-value `ArtifactKind` matrix, despite the tiny closed
   backing domain;
2. a bounded semantic fuzz oracle for the entry-name or retained JSON boundary;
3. a real `os.File` directory path in the package that owns selection;
4. a production-path positive/negative/neutral `LayerTriad`;
5. a complete data-flow inventory. The inventory contains only three
   `Validatable` assertions and omits the line-counter struct and intentional
   role classifications
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/contract_inventory_test.go:1-9`);
6. unknown regular-file format drift;
7. duplicate omitted-name accounting;
8. nil or overflow line-counter behavior;
9. bounded JSON allocation;
10. descriptor-relative/no-follow payload handoff;
11. benchmark allocation evidence for fixed-bound selection and line counting;
    or
12. a clean compiler-owned literal policy.

The literal issue is concrete:

- artifact-kind tests duplicate raw wire strings such as `"fuzz-corpus"` and
  `"fuzz-crasher"` instead of constructing accepted bytes from the typed enum
  and Core-owned token contract
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/artifact_kind_test.go:20-28`); and
- the directory-token test compares the Core constant to the duplicated raw
  literal `"fuzz"` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:fuzz/stream_test.go:65-70`).

Raw malformed JSON is appropriate for hostile syntax cases. Duplicating valid
protocol values is not.

## Primitive 2026 ownership and DAG

The clean reconstruction should remain small.

### Admit

Admit:

- `ArtifactKind` with Corpus and Crasher as the only valid values;
- exact Core-owned wire tokens and stable error identities;
- a narrowly named generated Go fuzz cache/crasher entry identity;
- a typed supported-toolchain/cache-format identity;
- a typed retention ceiling or typed caller limit bounded by a Core-owned
  absolute maximum;
- a bounded deterministic directory observation;
- explicit result state for complete, partial, unsupported-format, and failed
  observation;
- separate typed counters for retained, ignored, non-regular, over-limit, and
  unsupported-format entries;
- saturating checked arithmetic;
- partial facts on observation failure;
- defensive or iterator-based bounded output; and
- structure-to-structure handoff only.

All public request, result, policy, entry, and persisted types must own
`Validate()` at ingress, package crossing, execution, and external output.

Core should own only truly shared contracts:

- Fuzz error identities;
- artifact-kind protocol tokens;
- the exact Go cache directory token;
- a supported cache-format identity; and
- an absolute cross-package resource ceiling if Primitive genuinely mandates
  one.

The directory read batch and sort mechanics are package-local implementation
details and should not be placed in Core.

### Do not admit

Do not admit:

- `go test` process execution;
- fuzz duration, minimization, worker, seed, or scheduling policy;
- filesystem mutation, corpus promotion, or crasher cleanup;
- Witness bundle/index/manifest/ledger schemas;
- Peachfuzz sidecar/index/object/archive/finding schemas;
- content-addressed storage;
- generic logical-line counting under a fuzz-specific name;
- generic test discovery or ceremony statistics;
- arbitrary seed-corpus filename policy;
- compatibility aliases for archive names;
- a wrapper around `os.Open`, object stores, or product writers;
- loose maps or string blobs; or
- a result that calls a filename list captured evidence.

## Decision rationale and conditions

### Required clean migration

The migration must be direct:

1. implement the admitted Primitive 2026 types and Core contracts;
2. update Witness's eight production import sites and relevant tests to the new
   names;
3. update Peachfuzz's six production import sites and relevant tests;
4. move Witness's generic sidecar line accounting to its actual owner or a
   separately admitted generic primitive;
5. reconcile Peachfuzz's harvest ceiling with the Primitive retention
   contract without copied `128` values;
6. replace permissive 8..64 fixtures with the exact supported generated-name
   contract;
7. keep Witness and Peachfuzz product evidence schemas local;
8. remove old Fuzz names and dead paths; and
9. add no aliases, wrappers, or version bridges.

### Reconstruction acceptance gates

The implementation remains unready until all of these are green:

1. `ArtifactKind` is exhaustive across all 256 backing values, with typed JSON
   acceptance/rejection and receiver stability.
2. Generated entry-name tests prove the exact supported Go toolchain format,
   not an 8..64 compatibility range.
3. The directory observer has hostile tables meeting the protocol's
   positive/negative/neutral and serious-function thresholds.
4. A named `LayerTriad` crosses a real temporary directory and proves:
   exact retained names, explicit neutral absence, unknown regular-file
   handling, non-regular rejection/drop, partial read failure, and no fake
   completeness.
5. A semantic fuzz target mutates the non-tiny external name/JSON boundary and
   proves accepted `Validate()`/round trip and rejected stable identity with
   unchanged output. The closed artifact enum remains exhaustively tested
   instead of fuzzed.
6. Duplicate retained and duplicate omitted observations have an exact tested
   accounting unit.
7. Every arithmetic counter saturates or returns a typed overflow identity.
8. Every public request/result/policy and persistable type validates at its
   ownership boundary.
9. The data-flow inventory classifies every production struct by role.
10. Benchmarks report allocations for the directory selector and any retained
    streaming counter, with stable input size and no setup in the timed loop.
11. Production code remains streaming/bounded in corpus size and
    `gocyclo <= 10`.
12. Static architecture proof rejects production imports that would turn Fuzz
    into a process runner, filesystem mutator, object store, product schema, or
    campaign policy package.
13. Witness's real producer/writer/index/manifest/verifier path is green against
    the new Primitive revision.
14. Peachfuzz's real cache/object/index/archive and crasher-harvest paths are
    green against the new Primitive revision.
15. The Peachfuzz dependency checksum mismatch is resolved through a truthful
    published or local reviewed revision, never by bypassing module
    authentication.
16. The canonical Primitive gate, package tests, vet, static analysis,
    security analysis, complexity, coupling, and witness-lint gates are green.
17. No compatibility aliases or stale old API call sites remain.
18. No commit or push occurs before explicit user review and approval.

### Recon implications

The earlier free-text verdict was admission after redesign.

There are two independent production consumers of the same narrow,
product-neutral mechanics. The archive's typed artifact enum and bounded
deterministic selection are worth reconstructing. Witness proves their place in
a durable bundle and independent verifier. Peachfuzz proves their place in
content-addressed custody, explicit loss states, archival reachability, and
hostile crasher harvesting.

Do not copy the archive.

Rebuild Fuzz as a production primitive with:

- exact toolchain-format ownership;
- stable typed observation failures;
- explicit format-drift and drop accounting;
- no claim of payload custody;
- compiler-owned retention policy;
- real-filesystem and semantic-fuzz proof; and
- direct consumer migrations.

Keep product evidence, object custody, verification, campaign policy,
filesystem mutation, and generic line accounting with their real owners.

That is the smallest Primitive 2026 boundary supported by the evidence.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
