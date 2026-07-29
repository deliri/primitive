# Hostresource package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `hostresource`.
The archive is evidence, not authority. No archived production or test source
was copied.

The package addresses real needs already visible in consumers:

- Kernel needs a workload memory ceiling before applying product-owned Go
  runtime headroom;
- Peachfuzz checks output-disk pressure, Go-managed memory pressure, and Go
  runtime OOM evidence;
- Witness independently observes its Go-managed footprint, classifies runtime
  OOM only in conjunction with process termination, and reserves disk for
  terminal evidence; and
- Bug uses bounded directory streams for product-specific repository scans even
  though it does not need generic resource assessment.

The archived clean-cut implementation is a strong prototype. It distinguishes
five measurements that must not be conflated:

1. caller-available filesystem capacity;
2. Go-runtime-managed memory;
3. one Linux cgroup memory-limit observation;
4. logical regular-file tree extent; and
5. bounded detection of canonical Go runtime OOM banners.

It uses typed byte quantities, closed states, held filesystem capabilities,
checked arithmetic, fixed buffers, bounded directory batches, O(1) stream
processing, and stable error identities.

It is not admissible unchanged. Verified blockers include:

- `ObserveWorkloadMemoryLimit` is named as an effective workload observation
  but only reads fixed mount-root files and knowingly misses ordinary
  ancestor-scoped cgroup limits;
- a valid kernel `memory.max` value of zero cannot be represented;
- persisted `GoOOMEvidence` accepts byte counts impossible for the bounded
  classifier to produce;
- stderr banner presence is called OOM evidence without carrying the process
  termination fact that consumers need to prevent forgery;
- several exported proof values have no `Validate()` method at package
  crossings;
- percentages and regular-file counts remain raw integers;
- normal pressure states, unsupported capability, and observation failure all
  inherit `ErrHostResourceContract`, destroying error-axis separation;
- important package-local tokens, paths, buffer sizes, JSON fragments, and
  traversal policy are over-centralized in Core;
- error sites and cgroup paths still travel as raw strings; and
- the required native Linux and Windows matrices remain pending.

The 2026 direction is a clean reconstruction informed by the archive and all
four consumers. It is not a source copy or compatibility migration.

## Evidence boundary


### Source revisions and pins

| Source | Exact revision or Primitive pin | `hostresource` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`, `Harden capability inventory evidence`) | Present. First generalized implementation appears in `60e3acbf1a39e8f263abeb82a69a1db2528bef56`; clean replacement in `9a021858ff088ca35778cdf7c515b4ce30e73293`; workload-memory observation in `34bca74456891caa739fc9e706a9064d027a2c2f`. | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; inspected Hostresource and Core files are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed `go.mod` pins `0df2954a2d911a5d7d775691d023d569affa2c20` (`2026-07-22T21:25`, `2026-07-22T21:01-04`, `2026-07-22T21:00`), which contains the older Hostresource API. Dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` (`2026-07-27T00:33`, `2026-07-27T00:47-04`, `2026-07-27T00:00`), which contains the archive-head clean-cut package. | Broad pre-existing dirty migration. Committed and dirty APIs are different evidence. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` (`2026-07-22T07:30`, `2026-07-22T07:53-04`, `2026-07-22T07:00`), where `hostresource` is absent. | Pre-existing untracked testing protocol and ledger were observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` (`2026-07-20T10:59`, `2026-07-20T10:21-04`, `2026-07-20T10:00`), where `hostresource` is absent. | Only the pre-existing untracked ledger was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` (`2026-07-23T17:51`, `2026-07-23T17:17-04`, `2026-07-23T17:00`), which contains the older Hostresource API. | Only the pre-existing modified ledger was observed. |

Exact tree checks establish the generations:

- Kernel's committed pin contains `disk`, generic `memory`, tree, and
  `RuntimeOOM` sources but predates the clean replacement.
- Kernel's dirty pin contains the same Hostresource and relevant Core content
  as archive HEAD.
- Witness's and Bug's pins contain no `hostresource` tree.
- Peachfuzz's pin contains the older generic `MemorySnapshot`, `MemoryLimit`,
  `AssessMemory`, `RuntimeOOMEvidence`, and `DetectRuntimeOOM` generation.

The material archive history is:

- `60e3acbf1a39e8f263abeb82a69a1db2528bef56`: generalize host resource,
  durability, and shutdown;
- `d81af3c53364cce8bb57276130484b2ef25cc2a3`: harden resource durability and
  shutdown failures;
- `8c20a20138919f725269dd5a4d820bdf7081b77e`: consolidate typed exchange and
  runtime safety;
- `ecb0da7340f97e8ab8a80362fab053ac9473c6b7`: stabilize runtime intent APIs;
- `9a021858ff088ca35778cdf7c515b4ce30e73293`: replace Hostresource with the
  clean, narrow primitive set;
- `d259789e87bcadb829c5ffac72c6c91ccc604098`: centralize constants and close
  capabilities;
- `34bca74456891caa739fc9e706a9064d027a2c2f`: add typed workload memory
  observation; and
- `2717684bd111ce7fd2f4a23d9d4ed67af186beed`: promote checked percentage
  projection.

At archive HEAD, the package contains:

- 1,906 production Go lines;
- 2,272 test and fuzz Go lines;
- 4,178 total Go lines; and
- a 778-line specification.

Archive status records conflict. The specification itself remains `Draft for
human review` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:1-4`). The specification index
calls it a reviewed implementation with native Linux/Windows proof pending
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:20`). The completed ledger records human review
and completion of the clean-cut implementation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1999-2059`), while the pending ledger retains
the native cgroup, filesystem, APFS, NTFS, ReFS, quota, and reparse matrices
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:370-406`).

The unresolved evidence and defects below control 2026 admission.

## Capability ownership

The package should own observation, typed closure, and pure assessment of
specific host facts. It should never own the consumer action taken because of
those facts.

The reusable boundary is:

```text
typed observation request
    -> bounded native/runtime/stream read
    -> exact typed observed facts
    -> optional caller-policy projection
    -> closed assessment state
    -> consumer-owned action
```

The archive states the distinction directly:
Hostresource observes and classifies but does not pause work, terminate a
process, resize a runtime limit, reclaim memory, remove files, choose a product
budget, or decide restart policy (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:8-29`).

The reconstructed package should own:

- caller-available disk capacity for an exact held directory capability;
- total volume capacity and caller-available capacity as distinct typed facts;
- a caller-supplied typed disk-floor policy and exact pressure assessment;
- the exact Go soft-limit accounting metric
  `MemStats.Sys - MemStats.HeapReleased`;
- the current Go runtime soft limit as a typed observation;
- a typed percentage projection with exact checked ceiling arithmetic;
- a closed Go-managed-memory pressure assessment;
- an honestly named Linux cgroup observation with exact scope semantics;
- typed supported, unavailable, unlimited, zero-limit, and positive-limit
  workload states;
- bounded parsing of cgroup interface files through typed paths and sources;
- logical regular-file extent and typed regular-entry count beneath one exact
  root;
- descriptor/handle-rooted, no-link, no-reparse, no-mount-crossing traversal;
- fixed-batch and bounded-depth traversal;
- bounded O(1) classification of exact Go runtime fatal banners;
- a result name that distinguishes banner evidence from process-cause proof;
- complete `Validate()` methods on every exported request, policy,
  observation, assessment, and persistable evidence type;
- stable Core-owned error identities with independent contract, observation,
  unsupported, pressure, and process-evidence axes; and
- package-local structural, dependency, public-surface, O(1), and platform
  ratchets.

It should not own:

- pausing, aborting, retrying, cleanup, deletion, or restart;
- setting `GOMEMLIMIT` or calling `debug.SetMemoryLimit` with a new value;
- product percentages, disk floors, reserve sizes, or polling cadences;
- process supervision or termination classification;
- product record schemas or logging;
- physical allocated-size claims from logical file extent;
- a generic memory snapshot that merges Go, process, workload, Job Object,
  host, or operating-system pressure semantics;
- a filesystem service, probe registry, broad interface, or dependency
  container;
- a legacy cgroup-root shortcut behind an `effective workload` name;
- compatibility aliases for the retired generic API; or
- consumer-specific paths, constants, or errors.

## Archive evidence

### Archived strengths worth preserving

### Exact separation of memory scopes

The specification explicitly rejects a generic `MemorySnapshot` because
Go-managed bytes, resident/working-set bytes, commit or physical footprint,
cgroup or Job Object usage, host availability, and OS pressure are not the
same measurement (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:220-242`).

The clean API names `GoMemorySnapshot`, `GoMemoryAssessment`, and
`WorkloadMemoryLimit` separately. That fixes the older generation used by
Kernel and Peachfuzz, where `MemorySnapshot` could be read as a process or host
fact.

This semantic separation is foundational and should be retained.

### Caller-relative, handle-rooted disk capacity

`AssessDisk` validates context and request, opens and holds the exact resource
root, observes capacity through that held capability, closes it, and only then
assesses policy (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/public.go:15-31`).

Platform implementations preserve caller semantics:

- Linux uses descriptor `fstatfs`, `f_bavail`, and `f_frsize` with the
  documented `f_bsize` fallback
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/disk_probe_linux.go:10-35`);
- Darwin uses descriptor `fstatfs`, `f_bavail`, and `f_bsize`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/disk_probe_darwin.go:10-32`); and
- Windows calls `NtQueryVolumeInformationFile` on the held directory handle,
  using caller-available allocation units and checked multiplication
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/disk_probe_windows.go:26-69`).

Unix opens with directory, close-on-exec, and no-follow flags and records the
device identity (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/root_unix.go:14-38`). Windows compares
the lexical object with the opened object, rejects reparse roots, and records
the volume identity (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/root_windows.go:16-62`).

The pressure lattice is explicit: disabled at a zero floor, healthy strictly
above the floor, and reached at or below the floor
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/disk.go:64-115`).

This is materially stronger than path-based `statfs` helpers copied into
consumers.

### Exact Go-runtime-managed metric

`AssessGoMemory` reads `runtime.MemStats`, observes the configured soft limit
through `debug.SetMemoryLimit(-1)`, and constructs the snapshot from exact
`Sys`, `HeapReleased`, managed difference, and limit facts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/public.go:34-45`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/memory.go:67-103`).

The implementation:

- rejects impossible `HeapReleased > Sys`;
- treats `math.MaxInt64` as the disabled Go soft-limit state;
- calculates threshold with checked ceiling arithmetic;
- distinguishes healthy and reached at the exact threshold; and
- returns the complete assessment with `ErrMemoryLimitReached`, allowing a
  caller to inspect the observation while reacting to the typed state
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/memory.go:148-169`).

The spec accurately excludes RSS, physical footprint, cgroup usage, Job Object
commit, and host memory from that result
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:234-242`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:429-467`).

### Checked percentage projection

`GoMemoryPressurePolicy.TriggerBytes` validates policy first, validates the
positive observed limit second, invokes Core's overflow-safe ceiling
calculation, and validates the positive result before return
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/memory.go:10-52`).

The archive tests the projection through `math.MaxUint64`, monotonicity, the
whole-limit endpoint, and threshold adjacency. The arithmetic is stronger than
the repeated `limit / 100 * percentage` convention visible in consumers.

The percentage representation needs redesign, but the single checked
projection owner is a genuine reusable gem.

### Closed workload-limit state

`WorkloadMemoryLimit` has private representation and a `Validate` method that
closes state/source/value combinations:

- unsupported and unavailable require no source and no bytes;
- unlimited requires a real cgroup source and no byte value; and
- limited requires a real source and positive `ByteCount`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:16-117`).

V2 is authoritative when present, a malformed or unreadable v2 value fails
closed, and v1 is consulted only after v2 absence
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:224-269`).

Each virtual-file read uses one fixed maximum-plus-one buffer, validates hostile
reader counts, tolerates bounded zero-progress reads, checks context between
reads, and never grows with input
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:272-318`).

Those state and bounded-read mechanics should survive after the scope and zero
limit defects are corrected.

### Descriptor- or handle-rooted logical tree measurement

`MeasureTree` opens one exact root and walks it through a platform capability
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/public.go:53-69`).

The common walker:

- reads at most a fixed entry batch;
- retains a stack bounded by `core.FilesystemPathMaxComponents`;
- validates context on every step;
- uses checked byte and file-count accumulation;
- descends one child at a time; and
- joins every outstanding descriptor close error on failure
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/tree.go:41-159`).

Unix uses `fstatat` without following links and `openat` relative to the held
parent. It rejects mount/device crossing by comparing device identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/root_unix.go:60-106`).

Windows operates through `os.Root`, revalidates objects after open, rejects
reparse points, and enforces volume identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/root_windows.go:72-139`).

The result is honestly logical extent: sparse, compressed, cloned, and
hard-linked entries retain their documented logical semantics
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:485-520`).

This replaces the older `filepath.WalkDir` implementation used by Peachfuzz's
pin and is worth preserving.

### Bounded streaming OOM banner classification

`ClassifyGoOOM` accepts an exact declared byte length, allocates one fixed
buffer, reads exactly that extent, retains only matcher state, and returns the
examined length plus a closed kind
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/public.go:71-93`).

The matcher recognizes both canonical runtime fatal banners across arbitrary
chunk boundaries without retaining stderr
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/oom.go:94-143`). Reader count violations, premature EOF,
zero progress, context cancellation, and source errors remain explicit
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/oom.go:145-186`).

This is a substantial improvement over the older whole-slice
`bytes.Contains` API at Peachfuzz's pin.

### Private observation representation

Disk capacity, disk assessment, Go memory snapshot, Go memory assessment,
workload limit, tree usage, and OOM evidence retain private fields and expose
typed value accessors. Consumers cannot mutate returned state behind the
package's back.

The data-flow inventory names every production struct by role and forbids
untyped property bags (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:615-632`).

The missing validators discussed below must be added, but private
representation remains the correct default.

### Hostile-proof posture

The specification requires hostile tables for:

- disk arithmetic, caller-relative semantics, invalid roots, redirection,
  overflow, and native errors;
- exact Go memory thresholds and the complete percentage domain;
- cgroup precedence, parsing, bounded reads, hostile readers, cancellation, and
  source counts;
- real filesystem tree lifecycle, hard links, sparse files, symlinks, mount
  boundaries, disappearance, count/byte overflow, and allocation behavior; and
- OOM banner position, every chunk split, malformed near misses, exact extent,
  short sources, reader faults, and semantic fuzzing
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:634-740`).

The package also carries a production-struct inventory and public/dependency
ratchets. This proof ambition belongs in 2026 after the project-local testing
protocol replaces the archived protocol reference.

### Primitive dependency graph

At archive HEAD, Hostresource production imports only:

```text
hostresource
  -> core
  -> contextstate
  -> golang.org/x/sys/{unix,windows}
  -> Go standard library
```

It does not import Filestore, Objectstore, Exchange, Shutdown, a cloud SDK, a
database, or a product package. This matches its intended lower-level
observation role (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:31-50`).

Archive-wide direct-import inspection finds exactly one Primitive production
dependent:

```text
upgrade -> hostresource
```

Upgrade uses `AssessDisk` during Prepare, calculating a typed floor from
retained reserve, diagnostic output, artifact extent, document maxima, and
namespace reserve (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/prepare.go:197-225`). A refusal outcome
may carry the disk assessment (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/types.go:449-548`).

No other archived Primitive production package imports Hostresource.

The 2026 DAG should remain:

```text
core <- contextstate
  ^          ^
  |          |
  +---- hostresource
             ^
             |
          upgrade
             ^
             |
        product roots
```

`x/sys` is a platform implementation dependency, not a protocol owner.
Hostresource should not import Filestore merely because both touch the
filesystem. Filestore mutates durable state; Hostresource observes an exact
held root. Shared path and error types belong in Core.

## Consumer evidence

### Kernel

Kernel is the consumer that established the workload-ceiling requirement.

Its committed code at the `0df2954a...` pin performs cgroup discovery itself:

- explicit `GOMEMLIMIT` environment input wins;
- fixed v2 and v1 cgroup files are read through bounded Primitive durability;
- v2 `max` and large v1 sentinel values mean no usable limit;
- an 80% target is calculated through the older
  `hostresource.MemoryLimit.TriggerBytes`; and
- Kernel calls `debug.SetMemoryLimit`
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/memlimit.go:20-113`).

The current dirty migration moves cgroup observation and arithmetic into the
clean-cut package:

- `hostresource.ObserveWorkloadMemoryLimit` supplies the typed state and source;
- only the `Limited` arm yields bytes;
- `GoMemoryPressurePolicy.TriggerBytes` applies Kernel's 80% policy; and
- Kernel alone mutates and logs the Go runtime limit
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/memlimit.go:17-58`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/memlimit.go:79-96`).

That is the correct ownership split:

- Primitive observes and performs universal arithmetic;
- Kernel owns environment precedence, 80% headroom, logging, and runtime
  mutation.

Kernel also exposes the archive's scope defect. A service managed by systemd
may inherit `MemoryMax` from an ancestor cgroup without a private cgroup
namespace. The clean-cut observer deliberately returns no limit for that
deployment shape. Kernel then silently leaves the Go runtime unconfigured.

The dirty migration is not committed evidence and cannot be treated as
completed adoption.

### Witness

Witness's Primitive pin has no Hostresource package, so it implements three
related product behaviors locally.

First, its memory guard:

- parses an operator-owned positive byte limit;
- applies that value as `GOMEMLIMIT`;
- samples `MemStats.Sys - MemStats.HeapReleased`;
- fires one graceful-abort path at the product-owned percentage; and
- acknowledges that hard OOM still requires recovery
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/memory_guard.go:3-82`).

This confirms both the Go-managed metric and the non-ownership boundary:
Hostresource may return an assessment; Witness decides whether and how to abort
and seal evidence.

Second, Witness's subprocess classifier requires both:

- a signal termination; and
- a canonical runtime OOM banner in captured stderr.

A subject that prints the banner and exits normally is not classified as OOM
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/main.go:529-547`). Witness keeps only a fixed stderr tail
so fatal banners remain visible in O(1) memory
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/main.go:685-706`).

That correlation is a stronger gem than archive Hostresource's banner-only
`GoOOMEvidence`.

Third, Witness observes caller-available disk blocks for ledger pressure, but
its local implementation uses path `Statfs`, multiplies `f_bavail` by
`f_bsize`, saturates
overflow, and has no Windows implementation
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledgerfile/stat_unix.go:12-31`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledgerfile/disk_stats.go:5-15`).

Its product-level gem is not the probe. Witness preallocates a single terminal
reserve so disk exhaustion cannot erase the terminal evidence envelope
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledgerfile/ledger_terminal_reserve.go:3-22`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledgerfile/ledger_terminal_reserve.go:52-86`).
Hostresource should report capacity; Witness retains reserve and degradation
policy.

After a clean migration, Witness should retire copied runtime-banner constants,
raw `Statfs`, and duplicated Go-memory arithmetic. Its graceful-abort,
signal-correlation, tail-ring, and terminal-reserve policies remain downstream.

### Bug

Bug's Primitive pin has no Hostresource package and current production has no
runtime import of it. Protocol snapshots that contain Primitive Core files are
not a Hostresource call site and are not adoption evidence.

Bug does have a relevant bounded-directory mechanic. Its bucket reader opens a
root and directory handle, calls `ReadDir` with a fixed batch size, visits
matching product files, and stops only at EOF or a typed audit error
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/util.go:383-434`).

That corroborates the archive's rejection of whole-directory materialization.
The product traversal remains in Bug because it filters typed bucket files and
invokes a product visitor; it is not a generic logical-extent measurement.

Bug also uses `filepath.WalkDir` for a short-circuit query that checks whether
any durable state exists (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/repository_identity.go:154-183`). That should
not be forced through `MeasureTree`: existence and logical extent are different
contracts. The report therefore does not invent a Bug dependency merely to
justify the package.

Bug contributes no current disk-floor, Go-memory, cgroup, or OOM call site.

### Peachfuzz

Peachfuzz is the broadest committed Hostresource consumer, but its pin contains
the older API.

At each daemon cycle it:

- converts the configured output directory to a typed absolute path;
- supplies a product-owned disk floor; and
- returns the typed `AssessDisk` result error to pause/stop the cycle
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/cycle.go:70-93`).

Before work it:

- reads the current Go soft limit;
- creates the older generic memory assessment with a product-owned 90%
  threshold; and
- treats `ErrMemoryLimitReached` as the caller action boundary
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/resources_memory.go:9-30`).

After a fuzz process exits it:

- refuses OOM evidence for exit code zero;
- otherwise searches the retained stderr bytes;
- validates the result;
- gives runtime OOM precedence over harvested candidates; and
- classifies it as infrastructure failure
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/run.go:100-138`,
  `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/classify.go:16-61`).

The OOM fact is stored in the local run record and must agree with an
infrastructure outcome
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runrec/record.go:30-87`). Its wire adapter persists the
older enum as a raw string and parses it through Hostresource
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runrec/schema.go:14-20`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runrec/schema.go:37-80`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runrec/schema.go:82-104`).

Peachfuzz supplies four important findings:

1. disk and memory observations genuinely drive product execution;
2. product policy must remain outside Primitive;
3. OOM evidence affects durable accounting and therefore needs a stable,
   non-forgeable contract; and
4. clean-cut renames affect a persistence schema, so migration must revise the
   real schema and call sites without aliases or a dual decoder.

Peachfuzz's `exitCode != 0` guard is weaker than Witness's signal-termination
guard. An untrusted fuzz target can print an exact fatal banner and exit with
code 1, causing a candidate or target failure to be recorded as infrastructure
OOM. The reconstructed boundary should carry the exact process observation
needed for the claimed classification.

## Strong mechanics and proof

### Cross-consumer synthesis

| Capability | Kernel | Witness | Bug | Peachfuzz | 2026 owner |
| --- | --- | --- | --- | --- | --- |
| Caller-available disk capacity | No current use | Local path `Statfs` | No use | Older Hostresource use | Hostresource |
| Disk-floor policy | No use | Ledger-specific policy/reserve | No use |  Product floor | Consumer policy + Hostresource assessment |
| Go-managed memory metric | Startup limit configuration | Local watchdog | No use | Older Hostresource use | Hostresource observation |
| Workload/cgroup limit | Committed local implementation; dirty Hostresource migration | No use | No use | No use | Honestly scoped Hostresource observation |
| Go soft-limit mutation | Kernel | Witness | No use | Configuration/runtime | Consumer |
| Logical tree extent | Upgrade dependency only | No direct need | Product-specific bounded traversal | No current use | Hostresource |
| Runtime OOM banner scan | No use | Local tail classifier | No use | Older Hostresource use | Hostresource banner evidence |
| Process-cause classification | No use | Signal + banner | No use | Exit failure + banner | Process owner/consumer |
| Terminal action under pressure | No use | Graceful abort + disk reserve | No use | Pause/infrastructure outcome | Consumer |

The archive correctly centralizes mechanics that consumers otherwise copy:
checked capacity arithmetic, exact Go-managed metric, percent projection,
bounded tree traversal, and bounded banner matching.

The consumers show where the archive stops:

- resource observation does not own product action;
- an effective cgroup limit requires process membership and ancestor semantics;
- OOM cause requires process termination evidence in addition to bytes; and
- durable product schemas remain consumer-owned.

## Defects and blockers

### 1. `Effective workload ceiling` is not effective outside a private cgroup namespace

The specification uses the phrase
`the effective Linux cgroup workload-memory ceiling`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:8-15`) and says Kernel demonstrated the need for
that effective ceiling (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:244-248`).

Production reads only two fixed paths:

- `/sys/fs/cgroup/memory.max`; then
- `/sys/fs/cgroup/memory/memory.limit_in_bytes`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/hostresource_constants.go:29-32`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:244-269`).

The specification admits those paths identify the workload only under a
private cgroup namespace. In an ordinary host namespace they identify the
hierarchy root, so a process limited in a descendant or by an ancestor such as
a systemd unit does not observe that limit
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:275-286`).

The pending ledger leaves a namespace-independent observation undecided and
acknowledges that Kernel silently receives no soft limit in that deployment
shape (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:377-388`).

This is not an edge case hidden from the contract. It is a known mismatch
between the public name and the observed fact.

Required correction must choose one:

- implement `ObserveEffectiveWorkloadMemoryLimit` by parsing typed
  `/proc/self/cgroup` membership, resolving the mounted hierarchy, and folding
  the current cgroup and relevant ancestors under an exact documented rule; or
- narrow the API to a name such as
  `ObservePrivateCgroupNamespaceRootMemoryLimit` so callers cannot infer a
  general effective ceiling.

The first option is the useful 2026 capability. It requires typed mount,
membership, hierarchy, and ancestor facts; bounded streaming parsers; race and
namespace behavior; and native cgroup v1/v2/systemd/container proof.

No fallback may silently convert an unobserved limit into Unlimited.

### 2. A valid zero-byte cgroup limit is unrepresentable

The specification explicitly states that `memory.max` value `0` is a valid
kernel state but the package cannot represent it because `Limited` carries
positive-only `core.ByteCount`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:288-291`).

Production confirms the loss:

- canonical numeric parsing rejects zero
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:170-178`);
- the limited shape requires a valid positive `ByteCount`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:97-105`); and
- a zero file therefore becomes `ErrHostResourceObservation`.

Zero is neither malformed nor unavailable. It is an exact, operationally
critical limit that means no allocation headroom remains. Calling it an
observation failure erases the host fact and forces every caller to guess.

Required correction:

- add a closed zero/exhausted state, or represent a present exact limit with
  `core.ByteLength` plus a state that distinguishes zero from absence;
- make `LimitBytes` available for both zero and positive limited states through
  a typed arm;
- define the percentage projection's zero-limit refusal separately from
  observation; and
- hostile-test zero at the parser, result validation, projection, consumer
  boundary, and persistence/logging boundary.

### 3. Canonical OOM evidence can claim an impossible examined extent

Production ingress caps `GoOOMRequest.Length` at
`core.HostResourceGoOOMEvidenceMaximumBytes`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/oom.go:69-78`), and `ClassifyGoOOM` returns exactly that
declared length (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/public.go:71-93`).

`GoOOMEvidence.Validate`, however, validates only the kind and never checks the
examined extent (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/evidence_json.go:23-25`).

The archived test deliberately round-trips `math.MaxUint64` as
`bytes_examined`, even though the production classifier can never produce it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/extreme_hostile_test.go:142-160`).

This breaks proof-carrying provenance. Persistence can manufacture a
structurally canonical Hostresource result that is impossible at execution.
The test ratchets the bug.

Required correction:

- `GoOOMEvidence.Validate` must require
  `BytesExamined <= GoOOMEvidenceMaximumBytes`;
- JSON decode must call the same owner validation before assignment;
- the object JSON maximum must be schema-derived;
- maximum and maximum-plus-one hostile tests must use the production bound, not
  the full storage width; and
- a persisted evidence value must round-trip if and only if the classifier
  could have produced its shape.

### 4. Banner presence is not process-cause proof

`GoOOMEvidence.Kind == GoOOMGoRuntime` means only that at least one canonical
fatal banner occurred in the supplied bytes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:522-543`). The request carries a reader and
length but no process termination fact (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/oom.go:69-92`).

Any subject can write either exact banner. The matcher cannot establish that
the Go runtime emitted it, that the process died, or that memory exhaustion
caused termination.

Consumer evidence makes the missing fact concrete:

- Witness requires signal termination before accepting the banner
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/main.go:529-547`);
- Peachfuzz requires only nonzero exit, so a target that prints the banner and
  exits 1 is classified as infrastructure OOM
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/run.go:133-138`,
  `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/classify.go:49-58`).

Required correction:

- rename the result to `GoOOMBannerEvidence` and kind to
  `GoOOMBannerPresent/Absent`; or
- accept a typed process-termination observation and define the exact
  conjunction that authorizes `GoRuntimeOOMObserved`.

The first is the cleaner package boundary. Process supervision owns cause
classification; Hostresource owns bounded banner scanning. Consumers then
compose the two typed facts without claiming that bytes alone prove cause.

### 5. Exported proof values lack owner validation

The current doctrine requires validation at package crossing and external
output, owned by the type whose invariants are being enforced.

The clean-cut package exposes private-representation values but omits
`Validate()` on:

- `DiskCapacity` and `DiskAssessment`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/disk.go:38-61`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/disk.go:84-100`);
- `GoMemorySnapshot` and `GoMemoryAssessment`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/memory.go:67-103`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/memory.go:125-146`); and
- `TreeUsage` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/tree.go:28-39`).

`WorkloadMemoryLimit` does have complete owner validation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:57-105`), showing the intended
pattern. `GoOOMEvidence` has a validator, but it is incomplete as described
above.

Private fields prevent callers from inventing nonzero internals, but callers
can still create zero values, carry results across package boundaries, embed
them in structs, and persist projections. The receiving boundary cannot ask
the owner whether the value is valid. Upgrade works around this by checking
only the state accessor and comparing zero structs
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/types.go:487-496`).

Required correction:

- every exported observation and assessment implements `Validate()`;
- constructors and execution paths validate immediately before return;
- zero-value semantics are explicit for each type;
- a valid empty tree carries an intentional valid shape rather than relying on
  accidental equality with an uninitialized value, unless the contract
  explicitly makes zero the canonical empty fact; and
- consumers validate at every package-crossing, persistence, and external
  output boundary.

### 6. Percentage and regular-file count remain raw integers

`GoMemoryPressurePolicy` exposes `TriggerPercent uint8`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/memory.go:10-18`). Core's arithmetic likewise accepts a
raw `uint8`, guarded only by numeric constants.

`TreeUsage.RegularFileCount()` returns raw `uint64`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/tree.go:28-39`).

Both values are domain facts:

- percentage has a closed range and a semantic unit;
- regular-file count has overflow and result-shape rules.

The compiler cannot distinguish either from unrelated integers.

Required correction:

- Core owns a validated `Percent` type if percentage is shared across
  Primitive packages;
- arithmetic accepts `Percent`, not `uint8`;
- Hostresource policy carries `core.Percent`;
- Hostresource owns a typed `RegularFileCount` unless a genuinely shared Core
  count type is demonstrated; and
- consumers use typed constants for product percentages rather than raw
  numerals.

No aliases or wrapper overloads should preserve the raw API.

### 7. Error inheritance collapses contract, observation, unsupported, and pressure axes

Core defines:

- `ErrHostResourceContract` wrapping `ErrPrimitiveContract`;
- `ErrHostResourceObservation` wrapping `ErrHostResourceContract`;
- `ErrDiskCapacityUnsupported` wrapping `ErrHostResourceContract`;
- `ErrDiskFloorReached` wrapping `ErrHostResourceContract`; and
- `ErrMemoryLimitReached` wrapping `ErrHostResourceContract`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:156-160`).

As a result:

```text
errors.Is(observationFailure, ErrHostResourceContract) == true
errors.Is(diskFloorReached, ErrHostResourceContract) == true
errors.Is(memoryReached, ErrHostResourceContract) == true
```

A malformed request, an OS observation failure, an unsupported platform, and a
normal reached pressure state are not the same error axis. The specification
itself classifies floor/memory reached as
`typed state signals, not observation failures`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:462-467`), but their ancestry still labels them
contract violations.

Required correction:

- add a neutral `ErrHostResource` family root;
- make contract, observation, unsupported, and pressure siblings under that
  root;
- use specific disk and memory reached identities under a pressure identity;
- preserve standard filesystem, context, I/O, and native causes beneath the
  observation identity; and
- hostile-test positive and negative `errors.Is` relationships, not only
  expected positive matches.

Stable identity belongs in Core; rendered strings do not.

### 8. Core owns extensive Hostresource-local implementation detail

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/hostresource_constants.go:4-39` contains:

- directory batch size;
- every package enum token;
- OOM buffer and evidence limits;
- literal JSON head, separator, and tail fragments;
- enum JSON maximum;
- missing-path tokens;
- cgroup paths and tokens;
- cgroup v1 sentinel threshold;
- workload read maximum and zero-read tolerance; and
- both runtime OOM banners.

The specification says platform syscalls, runtime metric names, directory
batches, stream scanners, and observation machinery remain private to
Hostresource (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:43-46`). The implementation
contradicts that ownership rule.

Under 2026 doctrine:

- Core keeps shared byte, percentage, path, numeric, and error identities;
- Hostresource keeps local enum tokens, schema fragments, scan buffers,
  traversal batch size, source parser limits, cgroup grammar, and runtime
  banners;
- JSON ceilings are derived from local compiler-owned field and token
  constants;
- cgroup paths become typed Hostresource path contracts unless another
  Primitive package genuinely shares them; and
- consumers call the typed classifier rather than copying banner strings.

Witness currently duplicates both banners in its own Core because its pin
predates Hostresource. Clean consumer migration must remove those copies.

### 9. Cgroup paths and error sites remain raw string protocols

The workload reader behavior is:

```go
ReadValue(context.Context, string) ([]byte, error)
```

and `readWorkloadMemorySource` accepts `path string`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:119-121`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory.go:224-241`).
The operating reader passes that raw string directly to `os.Open`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/workload_memory_linux.go:13-28`).

Enum errors similarly accept raw names such as `"DiskPressureState"` and
`"GoOOMKind"` through `enumError(name string)`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/enums.go:24-34`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/enums.go:181-194`).

The stable sentinel remains compiler-visible, but the path/source and
operation/site dimensions do not. Typos or substitutions do not break the
build.

Required correction:

- use typed absolute virtual-file paths;
- use a typed cgroup interface source enum and a request struct for each read;
- type operation/failure sites or concrete structured errors;
- preserve stable identity with `errors.Is` and structured context with
  `errors.As`; and
- forbid string-matched tests and raw path literals in consumers.

Narrow behavior interfaces remain allowed. Loose string-bearing protocols do
not.

### 10. Unsupported tree measurement reports a disk-capacity error

On platforms other than Darwin, Linux, and Windows, the shared `platformRoot`
fallback returns `core.ErrDiskCapacityUnsupported` from:

- root open;
- close;
- disk capacity;
- directory open; and
- tree entry inspection
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/root_other.go:11-31`).

`MeasureTree` can therefore fail with `ErrDiskCapacityUnsupported` even though
it is not assessing disk capacity. A caller cannot distinguish
`tree measurement unsupported` from the unrelated disk capability.

Required correction:

- define a neutral typed platform-support state/error or operation-specific
  `ErrTreeMeasurementUnsupported`;
- ensure `errors.Is` reflects the actual failed capability;
- add cross-build tests for every supported and unsupported GOOS; and
- keep disk and tree observation errors distinct even if they share a private
  platform root implementation.

### 11. Disk capacity is artificially restricted to signed 64-bit

`newDiskCapacity` rejects total or available values greater than
`math.MaxInt64` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/disk.go:43-53`).
`DiskPressurePolicy.Validate` also converts a `core.ByteLength` floor to
`int64`, rejecting the unsigned upper half
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/disk.go:10-19`).

Disk capacity and floor comparison do not require signed arithmetic. Windows
native capacity is 64-bit unsigned, and Core byte types can represent the full
width. The package claims exact typed byte observations while silently
narrowing them.

Required correction:

- retain full `uint64` range in `ByteLength`/`ByteCount`;
- use checked unsigned multiplication and comparison;
- reserve `Int64()` conversion for an external boundary that actually requires
  signed values; and
- test `MaxInt64`, `MaxInt64+1`, and `MaxUint64` independently for capacity,
  availability, floor, and multiplication overflow.

### 12. Native lifecycle and filesystem proof is incomplete

The specification requires:

- native macOS disk, tree, runtime, and unsupported workload tests;
- native Linux filesystem, mount-boundary, and cgroup tests;
- native Windows NTFS and ReFS quota and reparse tests; and
- Linux and Windows cross-builds on every change
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:742-761`).

The pending ledger still requires:

- native limited/unlimited cgroup v1/v2 matrices;
- host-with-neither-interface proof;
- unreadable subtree, disappearance, mount boundary, and permissions on every
  supported OS;
- APFS clone/compression;
- Windows compression, quota, reparse, NTFS, and ReFS; and
- source-baseline re-review
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:370-406`).

The local interview reran host package gates only. It did not reproduce those
native matrices. Cross-build or unit success cannot prove descriptor, handle,
quota, reparse, mount, compression, or cgroup namespace behavior.

Implementation admission remains unready.

### 13. Consumer cutover is incomplete

Peachfuzz still compiles against the retired generic API. Kernel's committed
code also uses the retired `MemoryLimit`, while its clean-cut migration exists
only in a dirty working tree. Witness retains copied OOM banners and local
resource mechanics. Bug does not need the package.

The archive's own stable-surface contract says `AssessMemory`,
`MemorySnapshot`, `MemoryLimit`, `DetectRuntimeOOM`, `RuntimeOOMEvidence`, and
`RuntimeOOMKind` are retired with no compatibility spelling
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/SPEC.md:298-350`).

Required cutover:

- land and push the reconstructed `/v2026` package first;
- update Kernel and Peachfuzz to that exact pushed revision;
- replace real call sites and persistence schemas;
- remove superseded local mechanics where Primitive owns the capability;
- retain consumer policy and process classification downstream; and
- add no alias, shim, dual decoder, or local module replacement.

### Mechanical gate evidence at the inspected archive

The following commands were run against archived source at
`d046f7b675fcb797398d7cdc87b5504f43978056`:

| Gate | Result |
| --- | --- |
| `go test ./hostresource` | PASS (`0.557s`) |
| `go test -race -shuffle=on -count=2 ./hostresource` | PASS (`3.647s`) |
| `go vet ./hostresource` | PASS |
| `staticcheck ./hostresource` | PASS |
| production-only `gocyclo -over 10` | PASS; no findings |

The first attempted shell expansion for the production-only complexity gate
did not split newline-delimited filenames under zsh and therefore was not
accepted as evidence. The recorded result comes from the corrected NUL-delimited
`find ... -print0 | xargs -0 gocyclo -over 10` command.

These package gates establish local compilation and mechanical cleanliness.
They do not discharge semantic defects, consumer migration, or native
operating-system evidence.

No full-module gate is claimed because this assignment is a read-only package
interview and the archive has unrelated working-tree state.

## Primitive 2026 ownership and DAG

### Admit to `hostresource`

Admit after redesign:

- typed disk observation request, capacity, policy, state, and assessment;
- held-root platform observation;
- checked caller-available capacity arithmetic;
- typed Go-managed memory observation;
- typed Go soft-limit observation and disabled state;
- typed percent policy and exact ceiling projection;
- honestly scoped cgroup membership/hierarchy observation;
- bounded virtual-file parsers;
- complete workload-limit state including zero;
- logical tree extent plus typed regular-file count;
- bounded descriptor/handle-rooted traversal;
- bounded Go runtime fatal-banner evidence;
- canonical package-local evidence codecs where persistence is demonstrated;
- complete validators on every exported value;
- structured operation/site errors preserving stable Core identity; and
- package-local dependency, public-surface, O(1), data-flow, and platform
  ratchets.

### Admit to `core`

Core should own only genuinely shared contracts:

- `ByteCount` and `ByteLength`;
- a validated shared `Percent` and exact checked projection arithmetic;
- absolute file and directory paths;
- filesystem path component/depth contracts shared across packages;
- shared file-mode and numeric-overflow identities; and
- stable Hostresource error identities under a correct neutral family root.

Core should not own Hostresource's:

- enum tokens;
- OOM banners;
- JSON byte fragments;
- traversal batch size;
- OOM buffer/evidence sizes;
- cgroup interface paths or grammar unless another Primitive package truly
  shares them;
- cgroup sentinel classification;
- zero-read tolerance;
- operation/site labels; or
- schema field count and maximum formulas.

### Preserve in lower and adjacent packages

- Contextstate owns context ingress validation and terminal classification.
- Core owns universal numeric, byte, percent, path, and error contracts.
- Hostresource directly owns native read-only observation; it does not route
  through Filestore mutation capabilities.
- Upgrade may consume a Hostresource disk assessment but does not become its
  rule owner.
- Shutdown owns generic bounded cleanup execution, not pressure observation.
- Process supervision packages own exit/signal/wait facts; Hostresource owns
  banner bytes only.

### Keep in consumers

Consumers retain:

- percentages, floors, reserves, polling cadence, and headroom;
- environment-variable precedence;
- runtime mutation;
- pause, abort, restart, retry, and cleanup decisions;
- product logs and metrics;
- process supervision and termination classification;
- product-specific directory filtering and visitors;
- durable run-record schema and migration;
- customer-facing error presentation; and
- deployment assumptions such as private cgroup namespace.

### Required DAG

```text
                    core
                     ^
                     |
               contextstate
                     ^
                     |
               hostresource
                     ^
                     |
                  upgrade
                     ^
                     |
          product composition roots
          /        |          \
       Kernel    Witness    Peachfuzz

Bug imports Hostresource only if a demonstrated future need appears.
```

Platform implementation detail:

```text
hostresource -> x/sys/unix
hostresource -> x/sys/windows
```

There is no consumer-to-consumer edge and no product fact moves into
Primitive Core.

## Decision rationale and conditions

### Required admission proof

Before this report can become complete, the reconstructed package must provide:

1. A reviewed 2026 specification with exact ownership and non-ownership for
   each of the five observation families.
2. A decision on package cohesion: either retain one narrow Hostresource
   package with explicit intent verbs or split only if the resulting packages
   eliminate rather than duplicate shared native machinery.
3. A typed `Percent` contract and direct consumer updates.
4. Complete validators for every exported request, policy, observation,
   assessment, state, and evidence value.
5. A neutral Hostresource error family with independent contract,
   observation, unsupported, pressure, and evidence identities.
6. A correctly named and implemented effective cgroup observation, including
   `/proc/self/cgroup`, mounted hierarchy, ancestor folding, namespace
   semantics, v1/v2 precedence, and typed paths.
7. Exact representation of a present zero-byte cgroup limit.
8. Bounded streaming cgroup parsers with hostile zero-progress, count, EOF,
   permission, disappearance, and mutation tests.
9. Full-width unsigned disk capacity and floor arithmetic.
10. Native caller-relative capacity proof, including quota behavior.
11. Descriptor/handle-rooted tree tests for symlink, reparse, mount, device,
    hard-link, sparse, clone, compressed, disappeared, unreadable, count
    overflow, and byte overflow behavior.
12. Allocation proof showing tree memory is independent of entry count/file
    size within the fixed depth contract.
13. A banner-evidence API that does not overclaim process cause.
14. OOM evidence persistence whose valid extents exactly match execution
    production bounds.
15. Consumer composition tests showing signal/exit facts combine with banner
    evidence without forgery.
16. Package-local schema-derived enum and aggregate JSON ceilings.
17. Dependency, no-product, no-map, no-world-build, no-raw-path,
    public-surface, alias, compatibility, waiver, gocyclo, and coupling
    ratchets.
18. Native macOS/APFS, Linux filesystem+cgroup, Windows NTFS/ReFS,
    quota/compression/reparse, and cross-build evidence tied to the exact
    source revision, platform, and executed command set.
19. A clean Kernel cutover and Peachfuzz cutover against the pushed `/v2026`
    module, with retired APIs and copied constants removed.
20. All project-local canonical, race/shuffle/repeat, vet, static analysis,
    nilness, security, vulnerability, dead-code, complexity, witness, and
    sentinel gates.
21. Fresh independent review after implementation and evidence are complete.

Tests must follow `foundation@working-tree:_docs/testing_protocol.md:1-12`: typed fixtures,
hostile boundary tables, independent arithmetic and matcher oracles, real
filesystem/native proof, controlled contexts, no sleep synchronization, no
string-matched errors, and no fake filesystem standing in for lifecycle
behavior.

### Recon implications

The capability has real cross-consumer demand, but the archived implementation
requires redesign before it can support a package specification.

Admit the capabilities because:

- Kernel and Peachfuzz demonstrate real production demand;
- Witness independently corroborates the Go-managed metric, OOM banner, and
  caller-available disk needs;
- Upgrade has a real Primitive dependency on disk preflight;
- the archive has strong typed states, exact platform semantics, held-root
  observation, checked arithmetic, bounded streaming, and O(1) mechanics; and
- the package correctly leaves product action and policy downstream.

Reject the archive as-is because:

- the workload-limit name overstates a private-namespace-root observation;
- zero cgroup limits are erased as failures;
- impossible OOM evidence is admitted by canonical persistence;
- banner bytes are overclaimed as runtime-cause evidence;
- exported proof values cannot consistently validate themselves;
- raw percentages and counts bypass compiler ownership;
- error ancestry collapses normal states into contract violations;
- Core contains extensive package-local implementation detail;
- raw strings carry paths and failure sites;
- tree unsupported errors claim disk-capacity failure;
- disk observations are unnecessarily narrowed to signed range;
- native platform proof remains external; and
- committed consumers still depend on retired APIs or local copies.

No compatibility debt is justified. Kernel and Peachfuzz must update real call
sites and schemas in clean cuts after Primitive is pushed. Witness should
remove duplicates only when the new package fully replaces them. Bug should
remain independent until it has a demonstrated matching need.

The recon report is complete. The capability remains unadmitted until the
reconstructed specification, implementation, native evidence, consumer
cutovers, canonical gates, and fresh implementation review are complete.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
