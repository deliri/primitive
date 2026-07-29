# Filestoretest package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `filestoretest`.
The archive is evidence, not authority. No archived production or test source
was copied.

The package owns a real, narrow test capability: deterministic cumulative
write exhaustion at an exact byte boundary. An injected writer receives no more
than the configured capacity; the crossing call reports the exact accepted
prefix and `syscall.ENOSPC`; later non-empty writes report `(0, ENOSPC)`.
This lets a hostile test force a write barrier to prove its real partial-write
discipline without consuming an actual filesystem.

Independent consumer evidence validates the need:

- archived Filestore uses the helper on its real bounded-read path;
- Witness independently built the same cumulative ENOSPC writer before the
  Primitive archive did, then used it across real-file rollback, cleanup,
  hashing, buffering, ledger rotation, and reserve paths;
- Kernel has a local failure writer for buffered-flush and cursor-ordering
  proof that could use the typed ENOSPC capability;
- Bug proves committed-prefix hashing with a local pure-short writer, exposing
  a distinct adjacent fault shape; and
- Peachfuzz classifies exact `ENOSPC` and `EDQUOT` identities into
  product-owned pause policy but has not driven those identities through its
  persistence paths.

The archive is not admissible unchanged. Its byte contract uses raw `uint64`
despite Core already owning `ByteLength`; its invalid configuration reports a
production Filestore contract error; its exported writer has no `Validate()`;
the zero writer silently behaves like a configured full disk; typed-nil and nil
receiver behavior is unproved; its external observations are loose scalars; its
threshold subtests have meaningless generated names; its concurrency test has
no cancel path or timeout backstop; and its production-import prohibition is a
repository-local scan rather than a complete consumer lint contract.

The 2026 direction is therefore a small clean reconstruction, not a rename shim
or source copy.

## Evidence boundary


### Source revisions and exact pins

| Source | Exact revision or Primitive pin | `filestoretest` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`, `Harden capability inventory evidence`) | Present and identical to the package at Kernel's dirty pin. | `filestoretest/` and its cited Core/Filestore files are clean against HEAD. One unrelated untracked file exists at `core/api_http_boundary_hostile_test.go`. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` (`2026-07-23T04:01`, `2026-07-23T04:52-04`, `2026-07-23T04:00`, `Forbid disabled CSP in production`) | Committed `go.mod` pins `0df2954a2d911a5d7d775691d023d569affa2c20` (`2026-07-22T21:25`, `2026-07-22T21:01-04`, `2026-07-22T21:00`), where `filestoretest` is absent. Dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` (`2026-07-27T00:33`, `2026-07-27T00:47-04`, `2026-07-27T00:00`), where it is present. | Broad pre-existing dirty migration. The cited Exodus source is clean and belongs to committed HEAD. No Kernel source imports `filestoretest` at either observed worktree state. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` (`2026-07-24T15:52`, `2026-07-24T15:58-04`, `2026-07-24T15:00`, `protocol: preserve extracted Primitive contracts`) | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` (`2026-07-22T07:30`, `2026-07-22T07:53-04`, `2026-07-22T07:00`), where `filestoretest` is absent. | Only the pre-existing untracked `.ledger_pending.md` was observed. Cited source is clean. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` (`2026-07-24T15:52`, `2026-07-24T15:54-04`, `2026-07-24T15:00`, `protocol: preserve extracted Primitive contracts`) | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` (`2026-07-20T10:59`, `2026-07-20T10:21-04`, `2026-07-20T10:00`), where `filestoretest` is absent. | Only the pre-existing untracked `.ledger_pending.md` was observed. Cited source is clean. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` (`2026-07-24T15:52`, `2026-07-24T15:50-04`, `2026-07-24T15:00`, `protocol: preserve extracted Primitive contracts`) | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` (`2026-07-23T17:51`, `2026-07-23T17:17-04`, `2026-07-23T17:00`), where `filestoretest` is absent. | Only the pre-existing modified `.ledger_pending.md` was observed. Cited source is clean. |

The exact tree checks matter:

- the four committed consumer pins all predate the package;
- Kernel's dirty pin contains the four-file `filestoretest` tree;
- the package content at Kernel's dirty pin has no diff against archive HEAD;
- no consumer currently imports the Primitive package; and
- Witness's independent local predecessor is therefore consumer requirement
  evidence, not a downstream use of the archived implementation.

## Capability ownership

The admissible capability is:

> A test-only, deterministic, concurrency-safe writer capability that limits
> cumulative accepted bytes, preserves the exact underlying accepted prefix,
> and produces a stable filesystem-exhaustion identity at the configured
> crossing boundary.

The owner must define:

- a typed validated configuration;
- an exact non-negative capacity;
- exact cumulative accepted extent;
- the crossing-call result;
- the post-exhaustion result;
- underlying-writer error precedence;
- impossible underlying count handling;
- concurrency serialization;
- zero-length-write semantics;
- observation of one coherent typed state; and
- a compiler-visible declaration that the concrete type implements
  `io.Writer`.

This capability is test support even though it lives in ordinary `.go` files.
Ordinary files are required so tests in other packages can import it. That does
not make production use legitimate; production-import exclusion must be a
compiler-adjacent lint/architecture contract.

### Non-ownership

`filestoretest` must not own:

- production Filestore execution;
- path parsing, namespace confinement, staging, activation, sync, cleanup, or
  recovery;
- product file schemas or persistence policy;
- whether a consumer retries, pauses, degrades, rolls back, salvages, or exits;
- arbitrary error injection through a loose `error` field;
- call-count schedules when the proved shared contract is a cumulative byte
  boundary;
- filesystem quota policy;
- actual native filesystem behavior;
- crash simulation;
- time, goroutine, or process orchestration;
- a registry of consuming products or packages; or
- compatibility with `durabilitytest` or any consumer-local helper.

An `io.Writer` fault cannot simulate `Sync`, directory sync, rename, link,
close, quota reservation, or ambiguous activation. Those effects require
their owning package's typed test seam or native failure-injection proof.
The v2026 specification must say this directly so an ENOSPC writer test is not
misreported as a durability proof.

## Archive evidence

### Archive history and size

The current package has:

- 83 lines of non-test Go;
- 132 lines of Go tests;
- a 10-line package specification; and
- 225 total lines across the four package files.

The current identity entered the archive in two commits:

1. `52d9062da037fdf0bf8774bd9af3130ed0ffee17`
   (`2026-07-24T04:32`, `2026-07-24T04:58-04`, `2026-07-24T04:00`,
   `foundation: replace durability with filestore primitives`) cleanly renamed
   the old `durabilitytest` package and changed its identities to Filestore
   identities.
2. `40ded9c104a99cbc4b0b672cd7392901b468d1eb`
   (`2026-07-26T23:14`, `2026-07-26T23:02-04`, `2026-07-26T23:00`,
   `Harden Primitive comparative contracts`) added the ten-line
   `filestoretest/SPEC.md`.

The pre-rename helper at parent
`a00668941af086e23a29f50bcb330e41ad10cfd8` had the same basic implementation.
The clean rename removed the retired Durability names rather than keeping an
alias or forwarding package. That clean-cut instinct is worth preserving; none
of the retired identifiers should reappear in v2026.

Witness's local `internal/durabilitytest.ENOSPCWriter` predates the archive:

- `f38bd3e9a1d33b9f8b0a89713a611819f6c67cd8`
  (`2026-05-10T23:16`, `2026-05-10T23:43-04`, `2026-05-10T23:00`) introduced it; and
- `e5d17571a60d808c5720ab8417bd926b85c0fe2c`
  (`2026-05-10T23:22`, `2026-05-10T23:13-04`, `2026-05-10T23:00`) hardened its discovery and layer notes.

That chronology is important. Witness is independent prior operational
evidence, not confirmation produced by the archive itself.

### Archived strengths worth preserving

### Exact cumulative boundary

`ENOSPCWriter` stores one immutable capacity and one cumulative accepted count
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:23-28`). `Write` computes the remaining
capacity, sends at most that prefix to the injected writer, and never buffers
the caller's content (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:37-45`).

This is O(1) retained memory in total content extent. The wrapper neither copies
the whole input nor accumulates historical writes.

### Faithful crossing result

When the underlying writer accepts the entire allowed prefix and the caller
offered more, the wrapper returns the prefix count and exact
`syscall.ENOSPC` identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:49-58`).

That `(partial_n, ENOSPC)` shape is a meaningful writer fault, not merely an
invented error string: the archived helper returns the accepted prefix and
preserves the exact filesystem identity at the same boundary.

### Sticky exhaustion

Once cumulative accepted bytes equal capacity, later writes are refused before
the underlying writer is called
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:40-43`).

This lets a consumer prove it does not retry past a full-disk boundary or
silently advance accounting after the fault.

### Underlying error precedence

If the underlying writer itself returns an error while accepting the allowed
prefix, that error wins; the helper does not overlay synthetic ENOSPC
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:45-52`).

That distinction is necessary:

- partial plus `ENOSPC` is not the same as pure `io.ErrShortWrite`;
- partial plus `EIO` is not the same as capacity exhaustion; and
- a consumer must not manufacture an identity the lower layer did not report.

### Impossible-count containment

Negative counts and counts greater than the offered prefix become
`io.ErrShortWrite` without corrupting the cumulative counter
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:45-49`).

Although the exact v2026 error projection needs redesign, the archive correctly
refuses to trust an impossible `io.Writer` result.

### Concurrency serialization

One mutex protects threshold calculation, the underlying write, and cumulative
accounting as one transaction
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:37-60`).

This means concurrent callers cannot independently observe the same remaining
capacity and collectively exceed it. The helper also serializes access to an
underlying writer that may not itself be concurrency-safe.

### Typed ingress

Construction accepts `ENOSPCConfig`, not positional loose values, and the
configuration owns a `Validate()` boundary
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:11-21`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:30-35`).

The validation is incomplete, but the ownership shape is correct: caller input
is validated before the executable capability is returned.

### Hostile local tests

The archive's unit tests do more than check that code ran:

- capacities zero through payload length plus two prove both sides of the
  crossing boundary and the exact committed prefix
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc_test.go:14-48`);
- hostile writers exercise an underlying error, pure short write, negative
  count, oversized count, and full write
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc_test.go:50-86`);
- a concurrent campaign makes 1,024 write attempts through 32 goroutines and
  proves the sink and wrapper stop at exactly 257 bytes
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc_test.go:88-108`); and
- assertions use `errors.Is`, `bytes.Equal`, and direct got/want comparisons
  rather than error-string identity or assertion frameworks.

These are useful mechanics. They are not, by themselves, complete conformance
to the current testing protocol.

### Archived Primitive dependents

### Direct import graph

Exactly one archived Primitive file imports the package:

```text
filestore/stream_hostile_test.go
    -> filestoretest
        -> core
        -> standard library
```

The import is test-only at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/stream_hostile_test.go:13-15`.
No archived Primitive production file imports `filestoretest`.

The test:

1. creates a real file under `t.TempDir()`;
2. writes a payload larger than two Filestore stream buffers;
3. wraps an in-memory committed sink at
   `FileStoreStreamBufferBytes + 7`;
4. calls the real `filestore.Read` production API;
5. requires both `syscall.ENOSPC` and `io.ErrShortWrite`;
6. compares the `ReadResult`, wrapper state, and sink extent; and
7. proves the sink contains the exact source prefix.

The exact path is
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/stream_hostile_test.go:232-262`.
The production short-write projection under test is
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/read.go:175-186`.

This is good production-path proof for a destination writer failure during
Filestore read. It is not a native full-filesystem test and it does not exercise
Filestore write, sync, activation, or cleanup.

### Governance dependents

Core also carries compiler-visible governance facts for the test package:

- `PrimitiveFileStoreTestPackagePath` at
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_constants.go:61-71`;
- the module-path composition ratchet at
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/module_path_test.go:15`; and
- the production-import scan at
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/test_support_architecture_test.go:13-72`.

The package declares test-support capability but declares the Primitive layer
at `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/doctrine_contract.go:5-13`.
That mismatch is addressed below.

### Claimed versus located proof

The archive Filestore specification requires deterministic ENOSPC:

- on destination read at
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/SPEC.md:1146-1164`; and
- before a write, after a prefix, at exact capacity, and during sync at
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/SPEC.md:1183-1208`.

Only the bounded-read production path imports `filestoretest`. The package
self-tests prove the wrapper, but they do not prove Filestore's write/stage,
sync, activation, cleanup, or recovery behavior. The ledger's broad completion
claim at `archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1043-1048` is therefore wrapper
proof, not proof of every Filestore lifecycle requirement.

## Consumer evidence

### Kernel

Kernel's committed Primitive pin predates `filestoretest`, and no current
Kernel source imports it.

The strongest directly matching local use is Exodus migration:

- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/migrate_test.go:18-84` buffers encoded records, injects
  a failure at the underlying flush, and proves `migrator.run` returns the
  flush failure rather than silently discarding it;
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/migrate_test.go:104-152` proves the cursor file is not
  saved when output flush fails; and
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/migrate_test.go:87-102` implements the local
  call-count-based `countingWriter`.

The operational gem is not the local writer implementation. It is the
product-owned ordering invariant:

```text
encode into buffer
    -> flush output
        -> sync output
            -> only then persist cursor
```

A zero-capacity v2026 ENOSPC writer can drive the same buffered-flush failure
without `errors.New("disk full")`. Kernel's cursor ordering, migration types,
buffer choice, and error projection remain Kernel-owned.

The current tests contain two lessons for reconstruction:

- the shared helper must be composable under `bufio.Writer`, because exhaustion
  may surface at flush rather than at the caller's earlier buffered write; and
- adopting the helper must also replace the current error-string assertion at
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/migrate_test.go:77-84` with `errors.Is`.

Kernel also has many one-off failing writers for logs and output, but those are
not evidence for expanding this package into an arbitrary fault framework.

Admission implication: Kernel is a plausible test-only consumer after the
v2026 package is pushed, but it does not justify a call-count scheduler or
product migration API.

### Witness

Witness provides the strongest independent evidence.

Its local owner is `internal/durabilitytest`:

- package comments explicitly prohibit production imports and describe the
  write-barrier disciplines at
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/durabilitytest/enospc.go:1-26`;
- the writer owns cumulative threshold, accepted count, mutex, exact ENOSPC,
  and underlying-error precedence at
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/durabilitytest/enospc.go:35-131`; and
- unit tests cover below threshold, crossing, zero limit, cumulative calls,
  underlying error, and an `io.Writer` witness at
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/durabilitytest/enospc_test.go:11-185`.

Witness's helper is weaker than the desired Primitive contract:

- its constructor cannot reject a nil or typed-nil writer;
- negative limits are accepted as aliases for zero;
- counts and limits use raw signed integers;
- impossible underlying counts are not rejected;
- a negative underlying count can decrement `written`;
- pure short writes are accepted without normalization on the non-crossing
  branch; and
- it relies on a repository-specific allowlist for production-import
  exclusion.

Its integrations, however, contain important gems that Primitive's archive
does not:

#### Real-file rollback and accounting

`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/runwriter/runwriter_test.go:197-218` wraps a real
`*os.File`, so bytes accepted before ENOSPC exist on disk while the production
object retains its real sync/truncate/seek/close capability.

The production `BundleFile.Write` updates hash and byte count only for accepted
bytes (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/runwriter/runwriter.go:960-993`).
The hostile test proves:

- exact errno identity;
- absence of a false `io.ErrShortWrite` overlay;
- exact returned count;
- exact committed-prefix digest;
- exact byte accounting;
- sticky post-threshold refusal; and
- cleanup of the incomplete bundle

at `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/runwriter/runwriter_test.go:220-304`.

The record path goes further: `writeRecord` rolls a partial record back with
real truncate and seek before advancing hash or count
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/runwriter/runwriter.go:1143-1190`).

#### Partial-with-error versus pure short

Witness separately models:

- `(partial, ENOSPC)`, where the existing errno remains the identity; and
- `(partial, nil)`, where `io.MultiWriter` synthesizes
  `io.ErrShortWrite`.

The two fixtures and their purpose are explicit at
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/evidence/evidence_test.go:2750-2836`.
The real-file tests prove that a partial-with-error stops the fanout before hash
and counter writers advance, while cleanup removes the partial temp
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/evidence/evidence_test.go:2838-2948`).
The pure-short sibling proves `io.ErrShortWrite` without inventing ENOSPC
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/evidence/evidence_test.go:2950-3014`).

This is the clearest consumer case for a second small typed fault shape in the
v2026 specification. It does not justify one loose configurable `error`.

#### Buffered flush and sticky failure

Witness routes the ENOSPC writer through a buffered `SyncFile` seam and proves:

- ENOSPC during the initial buffered flush;
- preservation or absence of `io.ErrShortWrite` according to the actual lower
  result;
- sticky failure across later sync and close;
- exact bytes on the underlying sink; and
- the disk-full-at-first-byte boundary.

The test cluster is
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledgerfile/ledgerfile_test.go:3356-3725`.
The production flush classification is
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledgerfile/ledgerfile.go:624-661`.

#### Distinct non-writer fault capabilities

Reserve allocation and sync use separate test seams in production
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/runwriter/runwriter.go:560-598`).
This is a useful boundary lesson: a writer wrapper must not pretend it proved
allocation or sync behavior.

#### Policy that stays in Witness

Rollback, abort-publish, degraded reserve, terminal salvage, refusal to start,
ledger rotation, evidence hash/count meaning, and operator presentation remain
Witness-owned. Primitive should supply only the deterministic fault
capability.

After a reviewed v2026 package is pushed and Witness's complete dependency
closure is available, Witness can delete its local helper and update real test
call sites in one clean cut. No alias, wrapper, local `replace`, or dual helper
should survive.

### Bug

Bug's pin predates the package and no source imports a Primitive or local
ENOSPC helper.

Its closest local gem is a committed-prefix digest test:

- `shortGateWriter` accepts exactly a configured prefix and returns
  `io.ErrShortWrite`
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/gate_test.go:178-185`);
- `gateDigestWriter` is then required to hash only those accepted bytes
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/gate_test.go:163-175`).

This is a genuine reusable fault shape but it is not ENOSPC. Replacing it with
the archived helper would change the error identity and weaken the test.
The v2026 specification may admit a distinct typed pure-short capability based
on the combined Witness and Bug evidence. It must not make ENOSPC and
`io.ErrShortWrite` aliases.

Bug also performs direct exact file writes and copies, checking short counts
before sync at:

- `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:507-526`; and
- `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/public_artifacts.go:117-143`.

Those production paths currently lack an injected writer boundary, so a shared
helper cannot reach them without changing production shape merely for a test.
They should move to the admitted Filestore production capability rather than
gain a compatibility seam.

Admission implication: preserve the committed-prefix digest proof; do not force
Bug to import `filestoretest` until a real typed seam and matching error
contract exist.

### Peachfuzz

Peachfuzz's pin predates the package and no source imports it.

The directly relevant production behavior is product-owned failure policy:

- `pauseFailure` uses `errors.Is` to recognize
  `foundationcore.ErrDiskFloorReached`, `syscall.ENOSPC`, and
  `syscall.EDQUOT`
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/failure_severity.go:131-175`);
- the hostile table proves filesystem and quota exhaustion pause execution
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/failure_severity_test.go:12-83`).

The gem is the separation of mechanism and policy. The shared test package may
produce an exact standard identity; Peachfuzz alone decides that the identity
pauses work.

The current proof injects the errno directly into the classifier. It does not
prove that Peachfuzz's local persistence paths preserve ENOSPC through their
write, sync, close, staging, or cleanup layers. A future consumer cutover should
use the helper only where a real writer boundary exists and should add
production-path proof before deleting local assertions.

Admission implication: Peachfuzz does not need a `filestoretest` dependency for
the pure classifier test. It becomes a consumer only when the helper can drive
an actual persistence path without adding a test-only production wrapper.

### Cross-consumer synthesis

| Fault or invariant | Archive | Kernel | Witness | Bug | Peachfuzz | 2026 owner |
| --- | --- | --- | --- | --- | --- | --- |
| cumulative byte capacity | yes | local call-count substitute | yes | one-call prefix only | no | `filestoretest` |
| exact partial accepted count | yes | flush failure currently zero | extensive | yes | no | `filestoretest` supplies fact; tested package owns response |
| partial plus `ENOSPC` | yes | raw prose error today | extensive | no | classifier only | `filestoretest` plus standard errno |
| partial plus nil / pure short | only underlying hostile test | scattered local writers | extensive | yes | no | candidate distinct `filestoretest` fault shape |
| impossible writer counts | yes | not shared | local helper misses | not in cited fixture | no | `filestoretest` |
| buffered-flush surfacing | no helper integration | yes | yes | no | no | consumer production test |
| real-file accepted prefix | no; in-memory destination | yes for Kernel fixture sink | extensive | production direct paths | many local paths | consumer test |
| rollback/cleanup/salvage | no helper integration | cursor ordering | extensive | release policy | pause policy | consumer |
| digest/count only committed prefix | Filestore result/sink count | not cited | extensive | explicit | not cited | consumer |
| sync/rename/close failure | helper cannot express | local | separate seams | direct production | local | Filestore or consumer typed seam |
| retry/pause/degrade/exit policy | none | Kernel | Witness | Bug | Peachfuzz | consumer |

The corrected `v2026.0.0` topology keeps this narrow capability package-local.
It does not support a shared fault-injection framework.

## Strong mechanics and proof

### Testing-protocol proof

The current Primitive testing protocol was read completely before this
interview. Mechanical lint success is not treated as semantic proof.

### Rules the archive substantively satisfies

#### Evidence and non-vacuity

The wrapper tests assert exact counts, bytes, prefixes, identities, and
post-threshold behavior. Changing the boundary comparison, cumulative update,
error precedence, or mutex would make at least one test fail. This satisfies
the essential evidence requirement at
`foundation@working-tree:_docs/testing_protocol.md:149-170`.

The archive Filestore integration reaches the real `Read` implementation and a
real source file. It is honest production-path proof for the injected
destination boundary, consistent with
`foundation@working-tree:_docs/testing_protocol.md:862-893`.

#### Isolation and determinism

The helper unit tests mutate no filesystem. The Filestore integration owns a
`t.TempDir()` in the parallel test body. No test uses wall time, random data,
environment mutation, process output, or `time.Sleep`.

This is consistent with:

- isolation at `foundation@working-tree:_docs/testing_protocol.md:228-275`;
- parallel default at `foundation@working-tree:_docs/testing_protocol.md:277-305`;
- synchronization at `foundation@working-tree:_docs/testing_protocol.md:307-354`; and
- determinism at `foundation@working-tree:_docs/testing_protocol.md:356-380`.

#### Stable assertions

Tests use `errors.Is` for `syscall.ENOSPC`, `io.ErrShortWrite`, the injected
sentinel, and `core.ErrFileStoreContract`. They do not identify errors by
strings. This matches
`foundation@working-tree:_docs/testing_protocol.md:644-696`.

They use `bytes.Equal` and direct got/want checks, with no `reflect.DeepEqual`,
Testify, or hidden assertion helper, consistent with
`foundation@working-tree:_docs/testing_protocol.md:698-780`.

#### Boundary breadth

The threshold loop covers:

- zero capacity;
- every prefix through the payload;
- exact payload capacity;
- one and two above;
- the next write after exhaustion; and
- exact sink-prefix equality.

The hostile writer table separately covers negative, oversized, short,
erroring, and complete underlying results. The concurrent campaign proves the
aggregate exact capacity. These are meaningful boundary dimensions under
`foundation@working-tree:_docs/testing_protocol.md:466-516`.

### Rules the archive does not yet satisfy

#### Compiler-driven types

The protocol requires typed values, explicit validation, and compiler-visible
shape (`foundation@working-tree:_docs/testing_protocol.md:110-147`).

The archive has a typed config, but capacity and observation escape as raw
`uint64`; the writer has no `Validate()`; the zero writer is admitted; and no
typed state owns `written <= capacity`. The compiler cannot enforce the most
important result invariant.

#### Meaningful table names

The threshold test names subtests with
`string(rune('a'+capacity))`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc_test.go:18-21`).
Names such as `a`, `b`, and `c` do not name a boundary, failure mode, or
invariant. This violates:

- the review checklist at
  `foundation@working-tree:_docs/testing_protocol.md:83-102`;
- table shape at `foundation@working-tree:_docs/testing_protocol.md:382-460`; and
- helper/subtest vocabulary at
  `foundation@working-tree:_docs/testing_protocol.md:739-770`.

`witness-lint ./filestoretest` returned green because the generated expression
evades the semantic naming review. This is a concrete example of why a green
script is evidence only for the checks it actually implements.

#### Goroutine ownership

The concurrent test starts 32 goroutines and waits with `sync.WaitGroup`, but
it has no cancel path and no timeout backstop
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc_test.go:88-108`).

That violates the mandatory ownership proof at
`foundation@working-tree:_docs/testing_protocol.md:965-1005`.
The wait proves ordinary completion, not bounded exit if a future underlying
writer or wrapper regression blocks.

#### Red/green provenance

The implementation and tests entered the archive together. The located history
contains no recorded red state and no explicit pure-ratchet reason for the
current slice. That leaves `test/red-green-slice`
(`foundation@working-tree:_docs/testing_protocol.md:194-226`) unproved.

The reconstruction must either capture the hostile red state before the new
implementation or explicitly identify each already-correct behavior as a
contract ratchet.

#### Complete boundary proof

The archive does not locate tests for:

- typed-nil underlying writers;
- nil or zero `ENOSPCWriter` receivers;
- zero-length writes below, at, and after capacity;
- partial progress plus a non-ENOSPC underlying error;
- full progress plus a non-nil underlying error;
- `math.MaxUint64` capacity with a bounded input;
- coherent typed state validation; or
- reentrant/blocking underlying-writer behavior.

The current test suite is strong for its size, but it is not an exhaustive
trust-boundary matrix.

#### Production-import proof

The archive's structural scan is real and non-vacuous within Primitive:
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/test_support_architecture_test.go:13-72`.
It cannot prevent a downstream production `.go` file from importing the public
module path.

The v2026 package therefore needs both:

- the Primitive repository architecture ratchet; and
- a pinned consumer linter rule that rejects a test-support import from any
  non-`_test.go` file.

Documentation alone is an informal contract.

### Mechanical evidence at the archive

The following focused commands were run against the inspected archive:

| Command | Result |
| --- | --- |
| `go test ./filestoretest` | PASS (`0.260s`) |
| `go test -race -shuffle=on -count=2 ./filestoretest` | PASS (`1.246s`) |
| `go vet ./filestoretest` | PASS, no output |
| `staticcheck ./filestoretest` | PASS, no output |
| `witness-lint ./filestoretest` | PASS, no output |
| `find filestoretest -name '*.go' ! -name '*_test.go' -print0 \| xargs -0 gocyclo -over 10` | PASS, no function over 10 |
| `GOOS=linux GOARCH=amd64 go test -c ./filestoretest` | PASS; ELF test binary produced |
| `GOOS=windows GOARCH=amd64 go test -c ./filestoretest` | PASS; PE32+ test binary produced |

These commands establish compilation, race execution on Darwin, selected
static analysis, complexity, and Linux/Windows buildability. They do not close
the review-enforced testing gaps above and they are not native Linux/Windows
filesystem evidence.

## Defects and blockers

### 1. Capacity bypasses Core's existing byte type

`ENOSPCConfig.CapacityBytes`, `ENOSPCWriter.capacity`, and
`ENOSPCWriter.written` are raw `uint64`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:11-14`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:23-27`).
Both exported accessors also return raw `uint64`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:62-70`).

Core already owns `ByteLength`, whose zero is a valid exact extent
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/byte_length.go:9-27`).

Required correction:

- configuration uses `core.ByteLength`;
- cumulative observations use `core.ByteLength`;
- one typed state struct owns `written <= capacity`; and
- external output crosses the package boundary as that validated structure,
  not two loose scalar calls.

### 2. Invalid test configuration claims a production Filestore violation

Nil writer validation returns `core.ErrFileStoreContract`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:16-20`).

That identity says the production Filestore contract failed. It does not say a
test-support capability was misconfigured. A caller using `errors.Is` cannot
distinguish the two.

Required correction: Core owns a dedicated stable test-support contract
identity for this package or a deliberately shared typed test-capability
contract. It must remain distinct from production Filestore execution errors.
Standard `syscall.ENOSPC` and `io.ErrShortWrite` identities remain standard;
Core must not duplicate them.

### 3. The exported writer has no owner validation

`ENOSPCConfig.Validate()` exists, but `ENOSPCWriter` has no `Validate()`.

Consequences:

- `ENOSPCWriter{}` returns `(0, ENOSPC)` for a non-empty write because
  `written >= capacity` is true before the nil underlying writer is observed;
- the zero value masquerades as a legitimate configured zero-capacity writer;
- `(*ENOSPCWriter)(nil).Write` panics when it reaches the mutex;
- state corruption inside the package has no validating owner; and
- callers cannot validate the capability at a package crossing.

Required correction: the writer or its exported capability/state owner must
validate at construction, package crossing, execution, and external
observation. A valid zero-capacity configuration must remain distinguishable
from an invalid zero writer.

### 4. Typed-nil writers pass ingress validation

`c.Writer == nil` rejects only a nil interface
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:16-20`).
An interface containing a typed-nil pointer is non-nil and is accepted.
The first permitted write can then panic or execute arbitrary nil-receiver
behavior in the injected implementation.

The archive contains no typed-nil hostile case.

Required correction: define and prove the exact typed-nil boundary. Validation
must fail closed without calling arbitrary writer methods merely to discover
validity.

### 5. External observation is not structure-to-structure

`BytesWritten()` and `CapacityBytes()` expose two independent loose integers
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:62-70`).
No type owns their relationship and no `Validate()` can reject
`written > capacity`.

Required correction: return a typed immutable snapshot such as a package-owned
state value carrying Core byte lengths and an owner `Validate()` method. The
specification should choose the final name; the report does not freeze a public
identifier prematurely.

### 6. Zero-length write semantics are implicit

`Write` checks exhaustion before considering `len(data)`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:37-44`).
Therefore an empty write at an exhausted boundary returns ENOSPC, while an
empty write below capacity reaches the underlying writer.

Neither the package specification nor tests define whether that asymmetry is
intentional.

Required correction: the v2026 specification must define empty-write behavior
below, exactly at, and after capacity, then prove all three. The decision must
be compiler-visible through one owning implementation, not inferred from
branch order.

### 7. Underlying partial-error precedence is implemented but incompletely proved

The code increments `written` before returning an underlying error
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:45-52`), which is the right basic
shape when `n > 0`.

The hostile table tests only an underlying `(0, sentinel)` error
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc_test.go:53-65`).
It does not prove partial plus error, full plus error, or the resulting
cumulative state.

Required correction: hostile cases must prove exact count, state, sink bytes,
positive identity, and negative identity for each legal error/count
combination.

### 8. Impossible counts lose useful classification context

Negative and oversized counts become bare `io.ErrShortWrite`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc.go:45-48`).
That preserves a standard identity but does not retain a package-owned contract
identity distinguishing a broken injected writer from ordinary short progress.

Required correction: join the stable test-capability contract identity with
`io.ErrShortWrite` while returning a safe count and leaving state unchanged.
Tests use `errors.Is` for both identities. No error text becomes load-bearing.

### 9. The interface contract lacks a direct compiler witness

The archive package has no:

```go
var _ io.Writer = (*ENOSPCWriter)(nil)
```

The current Filestore test happens to compile when it passes the value as a
destination, but that is an indirect call-site witness. The package itself does
not pin the intended interface contract.

Witness's local predecessor does pin it at
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/durabilitytest/enospc_test.go:184-185`.

Required correction: retain a direct compiler witness in the owning package.

### 10. Generated subtest names conceal the boundary

The threshold matrix's subtests are letters generated from capacity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc_test.go:18-21`).
Failure messages include the number, but test discovery and selective execution
do not expose whether a case is zero, below, exact, or above.

Required correction: use a typed table with semantic names and explicit
capacity/want fields. The table should include exact, one below, one above,
zero, maximum unsigned, cumulative crossing, and post-exhaustion cases.

### 11. Concurrent goroutine proof can hang indefinitely

The concurrency test owns a wait path but no cancellation or timeout
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/enospc_test.go:88-108`).

Required correction: the hostile concurrency fixture must have:

- an explicit start/ownership mechanism;
- a cancel/release path for its controllable underlying writer;
- a wait path;
- a generous timeout used only as a deadlock backstop; and
- exact aggregate state proof under `-race`.

### 12. Red-state evidence is absent

The package's implementation and tests appeared together in the located
Primitive history. The archive does not record which hostile case was red
before the implementation or identify the suite as a pure contract ratchet.

Required correction: the reconstruction evidence records red/green per
behavioral slice or names the exact already-correct ratchet reason.

### 13. Production-import prohibition is only locally enforced

The archive specification says production packages `MUST NOT` import
`filestoretest` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/SPEC.md:3-10`).
The Core AST test enforces that inside one Primitive checkout
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/test_support_architecture_test.go:13-72`).

Nothing in Go's import system prevents a downstream production file from
importing the public package.

Required correction: the canonical lint contract must use the Core-owned
package path and reject it in non-test files in Primitive and every consumer.
The rule must have hostile analyzer tests for alias imports, dot imports,
blank imports, generated files, nested modules, and lookalike paths.

### 14. Doctrine declares the wrong layer

The package declares:

- `DoctrinePackageLayerPrimitive`; and
- `DoctrinePackageCapabilityTestSupport`

at `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestoretest/doctrine_contract.go:5-13`.

Core already defines `DoctrinePackageLayerTestSupport` and gives that layer its
own import rule
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/doctrine_contracts.go:13-22`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/doctrine_contracts.go:120-141`).

Required correction: the reconstructed package declares the test-support
layer directly. If capability and layer remain distinct contracts, the package
may also declare the test-support capability; the specification must explain
the distinction rather than carrying contradictory labels.

### 15. Archive proof is narrower than Filestore's specification

Only the Filestore bounded-read test imports the helper. The Filestore
specification also requires write/stage and sync-boundary ENOSPC proof.

An `io.Writer` helper can cover injected write boundaries. It cannot cover
sync, rename, link, close, cleanup, or ambiguous activation.

Required correction:

- use `filestoretest` only where the production contract genuinely crosses an
  `io.Writer`;
- use Filestore-owned typed test capabilities for other effects;
- retain native filesystem campaigns for native truth; and
- never cite the helper unit test as proof of a lifecycle it cannot reach.

### 16. Archive governance records disagree

The completed ledger says the support package and its ENOSPC tests are complete
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1043-1048`).
The live support-package ledger says helpers must still remain typed, hostile,
deterministic, and production-path equivalent
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:503-513`).
The package index still marks the local specification `Not written` even though
`filestoretest/SPEC.md` exists
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:51-58`).
The later freeze board leaves production-import and fault-surface indexing open
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:519-526`).

Required correction: v2026 has one live ledger and one package specification
status. A file's presence, a green helper test, and package admission are
separate typed states; prose indexes must not contradict them.

## Primitive 2026 ownership and DAG

### Package-local capability

Retain at the package that owns the hostile proof:

- a typed cumulative capacity fault configuration;
- a validated writer capability;
- exact accepted-prefix accounting;
- exact crossing and sticky-exhaustion behavior;
- impossible-count containment;
- underlying error precedence;
- concurrency serialization;
- coherent typed state observation;
- the direct `io.Writer` witness; and
- the package's test-support doctrine declaration.

The specification should also decide whether to admit one distinct pure-short
fault capability. Witness and Bug independently prove that `(partial, nil)` /
`io.ErrShortWrite` is load-bearing and must remain distinct from
`(partial, ENOSPC)`. If admitted, it must be a closed typed choice or distinct
typed capability, not a caller-supplied arbitrary error bag.

Do not add quota, EIO, close, sync, rename, crash, clock, random, or call-plan
world building without a separate located shared requirement.

### Shared Core facts only if later justified

Core owns only genuinely shared compiler contracts:

- non-negative `ByteLength`;
- the stable test-capability contract error identity;
- the v2026 module and package path used by import ratchets;
- doctrine layer/capability enum values;
- the linter-visible non-production import contract; and
- any repeated function-name contract required by the canonical linter.

Core does not own:

- helper mutexes or fields;
- threshold algorithms;
- test payloads or thresholds;
- consumer rollback/pause policy;
- consumer error labels;
- an inventory of consumers; or
- duplicate aliases for `syscall.ENOSPC` or `io.ErrShortWrite`.

### Preserve in Filestore

Filestore owns production filesystem execution and any typed seams required to
test:

- file write;
- allocation;
- full sync;
- directory sync;
- activation;
- reconciliation;
- close; and
- cleanup.

`filestoretest` may supply an injected writer to one of those seams. It does not
own the seam or the lifecycle response.

### Preserve in consumers

- Kernel owns flush-before-cursor ordering.
- Witness owns rollback, hash/count projection, abort-publish, reserve,
  degraded mode, salvage, rotation, and refusal policy.
- Bug owns committed-prefix release digest meaning.
- Peachfuzz owns pause severity and retry/control-plane behavior.

### Required dependency graph

```text
core -> filestore
core -> filestoretest
filestore tests -> filestoretest
consumer _test.go files -> filestoretest
consumer production -> never filestoretest
```

`filestoretest` must not import `filestore`. That would create a backwards edge
from a test support provider into the production capability it is designed to
test and would prevent other packages from using the generic writer fault.

Core must not import either package.

### Cutover order

1. Establish a second real consumer and obtain a reviewed root-catalog change.
2. Write and review a future package `SPEC.md`.
3. Implement the typed package and hostile proof.
4. Run the canonical Primitive gates and independent review.
5. Obtain explicit user review.
6. Commit and push Primitive.
7. Advance a consumer only when its complete Primitive dependency closure is
   available at that pushed revision.
8. Update real test call sites and delete the consumer-local duplicate in the
   same slice.

No local `replace`, copied source, wrapper, alias, dual helper, or compatibility
constructor is justified.

## Decision rationale and conditions

### Conditions for future reconsideration

### Contract proof

- typed config `Validate()` rejects nil and typed-nil writers;
- zero capacity is valid only in a constructed config;
- zero writer and nil receiver reject with the package's stable contract
  identity;
- capacity and state use Core byte-length types;
- typed state `Validate()` proves accepted bytes never exceed capacity;
- direct compiler witness pins `io.Writer`;
- the standard ENOSPC and short-write identities remain discoverable with
  `errors.Is`; and
- impossible underlying results carry both package contract and short-write
  identity without corrupting state.

### Hostile boundary table

Use semantic case names and include:

- empty write below capacity;
- empty write at capacity;
- empty write after an exhaustion event;
- zero capacity with non-empty input;
- one-byte capacity;
- capacity one below input;
- capacity exactly input;
- capacity one above input;
- multiple calls crossing the cumulative boundary;
- multiple calls filling it exactly;
- post-exhaustion retry;
- maximum unsigned capacity with a bounded input;
- underlying zero/nil;
- underlying partial/nil;
- underlying full/nil;
- underlying partial/error;
- underlying full/error;
- underlying negative count;
- underlying count one above offered;
- typed-nil writer; and
- writer panic or blocking behavior according to the explicitly owned
  containment contract.

Every case checks:

- returned count;
- positive error identities;
- negative error identities;
- exact underlying bytes;
- typed state;
- state validation; and
- whether the underlying writer was called.

### Concurrency proof

- many concurrent writes cannot exceed capacity;
- wrapper state equals underlying accepted extent;
- exact capacity is reachable;
- post-capacity writes do not reach the sink;
- the race detector is green;
- goroutines have a release/cancel path, wait path, and timeout backstop; and
- no assertion uses process-global goroutine counts.

### Production-path proof

- retain a real Filestore bounded-read integration;
- use a real file-backed destination where on-disk prefix proof is required;
- prove consumer hash/count state advances only for accepted bytes;
- prove rollback or cleanup through the real owning production path;
- separately prove pure-short and partial-with-error identities;
- separately inject sync/activation/cleanup faults through their real typed
  owner seams; and
- explicitly state which production layer each test did and did not reach.

### Architecture proof

- `filestoretest` declares the test-support layer;
- it imports only Core and the standard library;
- it does not import Filestore or consumers;
- Primitive production files cannot import it;
- consumer production files cannot import it under the pinned linter;
- analyzer hostile tests cover alias, dot, blank, generated, nested-module, and
  lookalike import cases;
- no retired `durabilitytest` path or symbol survives; and
- no consumer registry or copied product constant enters Core.

### Mechanical proof

- focused tests;
- race, shuffle, and repeat;
- `go vet`;
- `staticcheck`;
- `witness-lint`;
- `gocyclo <= 10`;
- global coupling coefficient zero;
- Linux and Windows cross-builds;
- canonical repository gate; and
- fresh independent review after every blocker is corrected.

A fuzz target is not automatically required. This helper has a small,
structured behavior space better exhausted by hostile tables. Add fuzzing only
if the final API introduces a genuine external parsing or protocol boundary
with an independent semantic oracle.

### Recon implications

The mechanics are reusable evidence, but the corrected `v2026.0.0` topology
keeps this helper package-local until a second real consumer proves a shared
boundary. The evidence remains useful because:

- exact cumulative write exhaustion is generic and product-neutral;
- it is O(1) in content extent;
- it enables deterministic hostile proof without requiring a genuinely full
  filesystem;
- archived Filestore already has a real production-path use;
- Witness independently implemented the same capability months earlier;
- Witness's integrations demonstrate substantial real-file value; and
- Kernel and Bug expose adjacent, concrete adoption opportunities.

Reject the archive as-is because:

- raw integer capacity and observation bypass Core's byte contract;
- invalid test configuration claims a production Filestore error;
- the writer and external state lack owner validation;
- zero and typed-nil states are unsafe or misleading;
- empty-write behavior is implicit;
- legal partial-error combinations and extreme states are unproved;
- impossible counts lack package contract identity;
- the direct `io.Writer` witness is absent;
- generated subtest names conceal boundaries;
- concurrent goroutines lack cancel and timeout proof;
- red-state provenance is absent;
- downstream production-import exclusion is not enforced;
- doctrine declares the Primitive layer instead of test support; and
- current evidence does not cover the broader Filestore lifecycle claims.

The reconstruction should preserve the archive's exact cumulative boundary,
error precedence, sticky exhaustion, bounded memory, and serialization. It
should add the compiler-owned types, validation, error identity, coherent
state, import ratchets, protocol-compliant hostile proof, and production-path
evidence learned from consumers.

The implementation remains unready until the v2026 specification is reviewed,
implementation and proof are complete, consumer cutover requirements are
sequenced, the canonical gates pass, and fresh independent review finds no
remaining blocker.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
