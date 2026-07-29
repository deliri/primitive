# Filestore package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `filestore`. Primary archive,
internal-dependent, consumer, and independent gap-review evidence is integrated.

## Evidence boundary

- Primitive archive: `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Kernel HEAD: `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`;
  committed Primitive pin `0df2954a2d91`, dirty pin `e8b7172161a4`.
- Witness HEAD: `b9629af57b7058b68982be5d3b282be440b1e76e`;
  pin `773add8ba0fc`.
- Bug HEAD: `39ce96242240d7174d562c90bb255860946595dc`;
  pin `388e593231a2`.
- Peachfuzz HEAD: `2b2d080c455edaadf88502c1c253845605a4336a`;
  pin `3f74d8fc35b4`.

The consumer worktrees are evidence-only and must not be normalized during
Filestore work. Kernel currently contains substantial unrelated tracked and
untracked work. Bug contains untracked `.ledger_pending.md` and
`_docs/testing_protocol.md`. Witness contains the known incomplete
`go.mod`/`go.sum` Primitive pin plus untracked `.ledger_pending.md` and
`_docs/testing_protocol.md`. Peachfuzz contains its modified
`.ledger_pending.md`. All are preserved.

The archive cleanly renamed the earlier `durability` capability to
`filestore`. Kernel and Peachfuzz contain older durability-era call sites;
Witness and Bug contain substantial local filesystem implementations. Those
are archaeological requirement sources, not compatibility requirements.

## Capability ownership

`filestore` turns bounded byte streams and namespace operations into the
strongest truthful persistence result supported by the host OS and filesystem.
It owns:

- rooted namespace operations over real `os.Root` and `*os.File` handles;
- exact streaming writes and bounded reads;
- staged atomic creation/replacement;
- file and containing-directory synchronization;
- explicit ambiguous-activation recovery;
- real append-handle creation and reopening;
- durable append-segment cutover;
- narrowly owned cleanup and single-entry removal; and
- typed persistence outcomes that retain native errors.

It does not own product file schemas, retention, artifact meaning, cloud
storage, ledger formats, rotation thresholds, dated layouts, upload schedules,
subscription segmentation, deletion policy, generic locking, recursive tree
policy, filesystem-capacity policy or observation, or why a path exists
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/SPEC.md:23`).

## Archive evidence

### Strong archived mechanics

### Typed write lifecycle

Writes use an explicit stage/commit/outcome lifecycle. `WriteOutcome` can report
that activation may have occurred while cleanup or durability remains
unresolved. `NeedsRecovery`, `Recover`, `Result`, and `Validate` make ambiguity
compiler-visible instead of collapsing it into one `error`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/types.go:436`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/architecture_test.go:201`).

Recovery reconciles descriptor and file identity before deciding whether to
finish or remove residue. Tests exercise ambiguous activation and recovery
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/reconcile_hostile_test.go:239`).

### Streaming and bounded memory

Content flows through readers and fixed buffers. Exact byte counts, short
writes, close errors, file sync, directory sync, and cleanup are separately
observed. Bounded reads have explicit ceilings. Recursive removal processes
bounded directory batches instead of loading the entire tree.

### Same-directory staging and durability

Stages live in the destination directory so activation does not cross mounts.
The package distinguishes atomic namespace visibility from durable persistence:
file data is synchronized, activation occurs, and the containing directory is
synchronized.

The archive bypasses `os.File.Sync` on Darwin to call `unix.F_FULLFSYNC`
without Go's documented fallback
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/sync_darwin.go:18`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/SPEC.md:185`).
That is archive-specific policy to reject. Current Go already owns the
platform implementation of `os.File.Sync`, including Darwin's
`F_FULLFSYNC` attempt and documented fallback. Primitive docks directly onto
that standard-library contract.

### Ownership capabilities

Root and file ownership use sidecar facts and held locks rather than a
process-local mutex. The mechanics are useful evidence about races, but the
lock sidecars, root-ownership protocol, private lock filenames, and custom
append wrapper are archive-specific designs to reject. A consumer that needs
single-writer election owns that product policy. Filestore returns real OS
handles and does not invent a competing lock or writer abstraction.

## Consumer evidence

### Kernel

Kernel uses the older durability surface and often recovers with the original
operation context. Once that context is terminal, cleanup may never run. The
new contract must accept a distinct live, bounded recovery context from the
consumer composition root; Filestore must not silently detach or invent a
cleanup lifetime
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:alfred/artifact.go:103`).

Its actual Filestore needs are intentionally ordinary: atomically and durably
replace generated Go/report files, and open real append handles for logs and
ledgers. `scout.GenerateReport` already consumes the older one-shot durability
operation, while `scout.AppendEntry` performs raw `O_APPEND` writes and
`File.Sync`. Kernel does not justify another abstraction layer above the
shared write and append primitives
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/report.go:22`,
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/ledger.go:57`).

### Witness

Witness is the original broad fuzz-evidence collection and ledger pipeline that
Peachfuzz later narrowed for independent collection. It collects fuzz corpus,
crashers, command sidecars, and evidence indexes, publishes them through its
run-bundle writer, and records the resulting projected facts into the ledger.
Its segmented ledger proves a minimal rooted append requirement: open a real
`*os.File` with append semantics, then let the ledger owner use the standard
`Write`, `Sync`, and `Close` methods and explicitly sync the parent directory.
The current product constants were verified in production rather than inferred:
the segment rotation threshold is exactly `10 << 20` bytes (10 MiB), disk
capacity is re-observed after each `1 << 20` bytes written, the default free
space floor is `100 << 20` bytes, and the terminal reserve is `64 << 10` bytes.
Those values remain Witness policy. The implementation separately tracks
lifetime bytes written and bytes in the current segment, so 10 MiB is a
per-segment cutover threshold, never a total-run ceiling.
Both long-lived collectors also need one product-neutral rotation effect:
synchronize and close the outgoing real handle, exclusively create the
caller-named incoming segment, synchronize it, and synchronize its parent
directory. Filestore owns that ordered OS effect while callers retain rotation
thresholds, names, envelopes, sequence continuity, replay, reserves, retention,
and terminal-record policy. Filestore must not interpose an `AppendWriter`
lookalike.
Its local durability code has useful operational lessons: explicit syncing,
staged replacement, rollback of published-but-unrecorded files, and attention
to custody boundaries. However, it relies on raw path strings, rename
conventions, unchecked or informal short-count assumptions, and a Darwin sync
layer now superseded by Go's own `os.File.Sync` implementation. The
requirements are valuable; the local API shape is not
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/durability/publish.go:42`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/durability/publish.go:138`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/runwriter/publish.go:98`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/fuzz_preservation_runwriter.go:1`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledgerfile/ledgerfile.go:79`).

### Bug

Bug's scale comes from independence rather than a shared Filestore lock: as
many as 10,000 agents may write concurrently, but each agent owns distinct
file names and must not block another through Primitive. Its current rooted
helpers prove the common physical operations directly: bounded regular-file
reads, synchronized writes, directory-chain synchronization, atomic replace
through a same-directory temporary, and race-free create-only publication by
hard-linking a fully synchronized temporary. These mechanics should collapse
onto shared Filestore operations. Bug's workflow-level coordination remains
Bug policy and does not justify a Filestore lock, queue, or scheduler
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/fs.go:44`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/fs.go:149`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/fs.go:195`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/fs.go:268`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/fs.go:407`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/lock.go:139`).

### Peachfuzz

Peachfuzz came after Witness and carries a narrower version of Witness's fuzz
collection and evidence-storage model so collection can run independently. It
adds a local content-addressed store and GCS archival workflow. Product-specific
digest identity, ledger/accounting, retention, reclamation, deduplication, and
GCS policy stay above filestore. Its real object-ingest path proves one shared
low-level need that ordinary predetermined-path publication does not:
target-late staging. Bytes must stream into a real temporary file while the
digest is computed, and only then can the final digest-derived target be named.
Its run-record ledger uses immutable create-only sequence files rather than
Witness's append-only segments. The two products therefore share rooted
publication, file and directory synchronization, activation state, cleanup,
recovery, and the need for bounded rollover during continuous operation, while
retaining their different ledger layouts and rotation policies above
Filestore.

Peachfuzz's intended continuous-run layout partitions its durable files by
date and performs its GCS archive pass nightly. Its typed `config.json` may
expose `deleteAfterUpload` as premium-subscription segmentation. That field is
consumer policy, not a Filestore option:

- when false, a GCS-confirmed local file remains present and collection
  continues until the configured real disk-safety floor is reached;
- when true, Peachfuzz may remove only a closed dated file whose exact bytes
  have been uploaded and verified and whose archive custody watermark has been
  durably advanced; it must then durably synchronize the local parent
  directory;
- the active file and every unconfirmed or unwatermarked file are never
  eligible for removal.

The current Peachfuzz config does not yet contain `deleteAfterUpload`; it
currently contains a 24-hour archive interval, a 5 GiB disk floor, and an older
`maxRunRecords` retention limit. The later consumer migration must add the
typed field and remove the arbitrary lifetime record ceiling. A five-year run
on a sufficiently large SSD must continue: total durable signal may grow
without bound while each file, directory generation, scan, recovery action,
and memory buffer remains finite and bounded. The sequence trie and current
pruning behavior are historical implementation evidence, not the future
retention contract.
Its generic effects still use the older durability/stage sweeper model, and its
local flock behavior is product policy rather than a Primitive locking
contract
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/store/store.go:104`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/store/store.go:292`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/durable/sweep.go:17`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/durable/sweep.go:49`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/lock/lock.go:47`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/lock/lock.go:139`).

## Strong mechanics and proof

### Explicit platform truth

The specification distinguishes Linux, Darwin, and Windows contracts instead
of presenting one fictional portable durability guarantee. Unsupported,
permission, I/O, containment, cleanup, and ambiguous states are meant to retain
stable typed identities.

### Primitive-internal consumers

Archived consumers include `callbudget`, `rate`, `register`, `status`,
`submission`, and `upgrade`. Their use reveals whether callers respect
`WriteOutcome`:

- `callbudget` and `upgrade` perform recovery
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store.go:718`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/storage.go:85`);
- some `status` and `submission` branches make unresolved recovery effectively
  unreachable (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:229`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:408`);
- `receipt` attempts recovery with the original, possibly terminal operation
  context (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_io.go:147`);
- `register` does not consistently recover (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:1012`);
- `rate` can leave stage residue (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/store.go:304`).

The capability is therefore stronger than several integrations. Primitive
2026 must test the crossing, not merely the filestore package in isolation.

## Defects and blockers

1. **The confinement claim exceeds the implementation.** The specification
   requires root identity to be established and rechecked and calls for strict
   no-symlink/no-mount traversal. Linux calls `openat2` unconditionally and
   provides no `ENOSYS`/unsupported descriptor-walk fallback at all; operations
   are nonfunctional where `openat2` is unavailable
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/openat_linux.go:10`,
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/SPEC.md:126`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/SPEC.md:413`). Darwin's stronger rename flags are not
   fully used. Windows still has
   lexical/`Lstat`-to-open race exposure
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/SPEC.md:413`).

2. **Exact creation mode is not guaranteed.** Supplying a mode to create is
   affected by process umask. Where the contract promises exact private mode,
   creation must be followed by descriptor-owned mode verification/correction,
   with typed failure.

3. **Lock failure identity is overcollapsed.** Several ownership and append
   paths wrap every lock failure as `core.ErrFileStoreBusy`
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/append.go:80`,
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/ownership.go:34`). Permission, descriptor, unsupported,
   containment, and I/O failures are not contention and must retain distinct
   stable identities.

4. **Native hostile proof is below the specification.** Missing coverage
   includes symlink/mount swaps, root replacement, open/rename races, filesystem
   feature absence, delayed writeback, permission matrices, process death at
   each transition, native Windows identity, Darwin rename flags, and Linux
   fallback behavior.

5. **Consumer recovery is inconsistent.** A write API that exposes unresolved
   state is only safe if every call site exhaustively handles it. Compiler-owned
   structure should make ignored recovery difficult, and crossing tests must
   prove all Primitive-internal consumers.

6. **Detached cleanup needs a hard bound.** Internal use of
   `context.WithoutCancel` prevents a cancelled parent from abandoning residue,
   but every detached operation must derive its own typed deadline
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/write.go:148`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/write.go:375`).

## Primitive 2026 ownership and DAG

- `core` owns shared path types, byte extents, file modes, operation/result
  contracts, and stable error identities.
- `contextstate` owns context ingress and hostile terminal observation.
- `filestore` composes real Go and OS filesystem primitives into the smallest
  rooted, streaming, synchronized, recoverable physical effects.
- `hostresource` owns filesystem-volume capacity observations. A long-running
  consumer combines those observations with its own durable byte accounting
  or the real `*os.File.Stat` result; Filestore owns neither the counter nor
  the safety-floor decision.
- product packages own file content schemas, append records, byte counters,
  rotation triggers, names, dated directory selection, retention, digest
  meaning, custody watermarks, GCS operations, subscription policy, and
  single-writer election.

The implementation should retain streaming O(1) behavior and explicit
ambiguous outcomes. It must not add a compatibility layer for `durability`,
accept raw arbitrary paths where a typed root-relative path is required, or
reduce recovery to an error string. It must not implement an in-memory
filesystem, replacement `fs.FS`, virtual path language, custom reader/writer
lookalike, wire enum set, lock sidecar protocol, recursive removal engine,
retry framework, transaction engine, or other filesystem world-building.

## Decision rationale and conditions

### Conclusion

The archive contains valuable evidence: typed ambiguous outcomes, rooted
target-late staging, file-plus-directory synchronization, bounded streaming,
real append accounting, and recovery. It is not promotion-ready.
Its stricter-than-Go platform layer, lock sidecars, custom reader/writer
wrappers, wire enums, recursive removal surface, and Primitive-owned append
ceilings are rejected. The redesign must prove rooted confinement through
Go's real primitives, exact ownership on every handle and residue path,
bounded work, native error reachability, truthful ambiguous activation, and
the long-running Witness/Peachfuzz consumer crossings.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
