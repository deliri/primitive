# Filestore package sweep — chronological evidence

Current status: [author closeout and release review](closeout_review.md).
The sections below retain chronological checkpoints; their pending notes describe
those recorded snapshots, not the final outstanding-work list. Repository-wide
gates remain deferred. Local package race/lint/verification results are in the
closeout report. No continuation changes have been committed or pushed.

The full 2,214-line local `_docs/testing_protocol.md` was read during the ongoing
sweep, with table, earned-row, layer-triad, handoff, exit and evidence rules
revisited before these edits. Protocol SHA-256:
`dd83cd7f62c172092546dab6b7c7d5c59753b5e8ae784631a94cff4d6d48126c`.

## Unchanged baseline

`baseline-package.json` records 808 passing Go events, zero failures/skips and
82.8% statement coverage. Event counts include parents and seeds, not earned rows.
All three existing benchmark functions ran separately for 30 seconds, CPU=1,
with same-invocation CPU/memory profiles and retained binaries:

| Workload | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Public lexical sparse walk | 21,994 | 4,264 | 21 |
| Public native sparse walk | 21,545 | 3,112 | 20 |
| Internal lexical sparse directory | 20,636 | 3,768 | 12 |

These are actual before measurements, not the older allocation-only summary.
Native calls dominate the sampled CPU. The public lexical allocation profile
attributes 54.61% to Go `os.File.readdir` and 26.84% to the package's lexical batch
storage. Raw evidence stays local in `testdata/test-upgrade-20260907`; JSON and
Markdown reports are intended for a later explicitly approved commit. The initial
source scope omitted the Filelock test-fixture dependency; the explicit baseline
supplement binds its unchanged Git revision and exact source bytes. Subsequent
captures include it and the scope configuration itself.

## Reproduced production defects and repairs

- Activation binding compared path and extent but omitted the exact rooted
  capability and permissions. The new table exhausts all subsets of those four
  agreement comparisons plus receipt presence, with nonempty/empty producer
  controls and explicit primary classes. Six failing rows exposed two missing
  checks. The corrected binding passes all 64 rows and verifies downstream
  activation plus no foreign-root effect.
- Stage namespace identity used `Stat`, allowing a symbolic link to borrow its
  target inode's receipt and a dangling link to become apparent absence. The
  42-row table crosses six real effect doors with seven namespace observations,
  including genuine hard links. Twelve failing rows expose these shared
  distinctions. The corrected `Lstat` checks preserve foreign entries and exact
  archived bytes. This is a shared defect across effects, not twelve regressions.
- Recovery checked inode identity without rechecking observed extent and mode.
  Thirty failing rows cover the two missing checks over three real landing
  positions and both install modes. Commit, Recover and OpenStagedRead now use
  one private owning observation validator.
- Replacing two names already linked to one inode can make Go rename return
  success without consuming the stage. A named recovery row found it; a separate
  four-row Commit/Recover table reproduced both replace paths while preserving
  exclusive-create conflict and create-recovery controls. Replacement now settles
  a retained owned stage through the existing identity-checked cleanup path.

The four new tables together pass 152 Go events. The broader activation and custody slice also passed: 307 Go events, no failures or skips. These are identity/extent/permission
receipts, not content hashes or a product workflow. Go still owns the real
files, rooted capability, stat, link, rename, synchronization and close operations.
Namespace observations are snapshots; these wrappers do not invent atomic
multi-name transactions against unsynchronized external writers.

## Current closeout

The legacy sweep, final package verification, platform compilation and profile
reporting are recorded in [closeout_review.md](closeout_review.md) and
[final_benchmarks.md](final_benchmarks.md). User review/commit approval,
repository-wide release gating and independent platform acceptance remain
outside the local author completion claim.

## Bounded stream continuation

Before the stream production edits, three new fixed binary copy workloads ran
for 30 seconds each, CPU=1, with CPU/memory profiles and retained binaries from
the same invocation. Reader and destination storage are reused; exact byte
comparison and receipt checking are part of the declared workload.

| Bounded copy workload | Before ns/op | Before B/op | Before allocs/op |
| --- | ---: | ---: | ---: |
| 128 bytes | 2,094 | 32,768 | 1 |
| 32 KiB | 5,053 | 32,768 | 1 |
| 1 MiB | 61,490 | 32,768 | 1 |

`before-stream-copy-*` records and analyses retain the measurements. Allocation
profiles attribute effectively all copy scratch allocation to `copyBounded`.
CPU samples contain substantial `runtime.kevent`; no application-CPU speedup is
inferred from those samples. Cumulative allocated gigabytes are not resident
memory. After measurements are still pending.

`red-stream-go-contract` records 26 failing behavioral rows (29 failure events
including three parents). The strengthened existing Stage/Write table removes
extra headroom that hid exact-limit failures for fragmented Go readers. New
local tables attack actual versus declared EOF, dishonest optional `Len`, Go
LimitedReader/SectionReader boundaries, joined EOF/native errors, invalid read
counts, no-progress thresholds, observed extent changes, short writes and exact
acknowledged prefixes. Valid writer behavior is compared directly with Go
`io.Copy`; impossible writer counts retain Primitive's stable short-write class.

The custom copy/retry loop and remaining-length type assertions are removed.
Go `io.Copy` owns transfer, `io.LimitedReader` owns the ceiling and Go's small
scratch-buffer choice, and `io.ReadFull` owns a bounded one-byte EOF probe. Private
Reader/Writer adapters enforce mechanical context, count, progress and error
identity checks. They deliberately expose no transfer shortcut that could skip
those checks. The architecture scanner admits only the compiler-visible exact
Go methods and has an exhaustive nine-row receiver/method cross-product guard.
No caller-owned reader/writer is closed. A generic blocking Reader must still
be released by its owner; context does not interrupt an arbitrary Go Read.

`red-stream-signed-domain-corrected` records six failing rows across Read,
Stage and Write: their unsigned limits exceeded Go's signed receipt domain.
Their owning Validate methods now use Core's checked Int64 conversion before
any effect. The initial `red-stream-signed-domain` attempt was a test build
error, not a production red proof. `green-stream-go-contract` passes 108 Go
events covering the repaired stream and existing cancellation/error paths.
The initial `checkpoint-stream-package` attempt was an architecture-test build
error while replacing a raw method-name constant; it is also not a production
failure. Full package verification is continuing under a distinct receipt.

### Stream checkpoint results

The corrected full-package receipt `checkpoint-stream-package-corrected` passes
1,033 Go events, zero failures/skips, 83.4% statement coverage. The later two
new semantic fuzz callbacks pass all 12 seeds (14 Go events including parents).
The initial public fuzz-seed attempt mistakenly expected a recovery request on
successful Write; the actual documented API returns one only for unresolved
failure. That test-oracle error is retained in `stream-semantic-fuzz-seeds`, and
the corrected callback proves a zero recovery request plus exact durable bytes.
It is not counted as a production defect.

All 12 deliberate production mutations were killed by behavioral failures;
`stream-and-activation-mutation-analysis.json` records exact replacements and
selected tests. Seven disable activation/custody safeguards. Five erase EOF
error handling, overflow refusal, exact no-progress threshold, invalid-count
native cause, or acknowledged write prefix. Both new fuzz callbacks kill a
semantic mutation inside their own callback. No build failure counts as a kill.

| Bounded copy workload | Before ns/op | After ns/op | Before B/op | After B/op | Before / after allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| 128 bytes | 2,094 | 174.1 | 32,768 | 241 | 1 / 5 |
| 32 KiB | 5,053 | 5,127 | 32,768 | 32,881 | 1 / 5 |
| 1 MiB | 61,490 | 61,965 | 32,768 | 32,881 | 1 / 5 |

`stream-benchmark-comparison.json` binds both sides to their command records,
samples and hashed artifacts. The unchanged workload observes exact bytes and
receipts. Small-copy time is 91.7% lower and allocated bytes 99.3% lower. The
larger copies were 1.5% and 0.8% higher in time in these single 30-second passes;
no large-copy speedup or statistically settled regression is claimed. Adapters
add a fixed 113 allocated bytes and four allocations per call. Large transfer
scratch allocation is now in Go `io.copyBuffer` and remains independent of total
stream extent; no pool or custom runtime was introduced. CPU profiles remain
poller-heavy and cannot substantiate an application-CPU speedup on their own.


## Custody synchronization continuation

The stream fuzz passes completed successfully: 566,316 bounded-copy callbacks
and 3,006 real Write/Stage/Commit/Read callbacks, each in its own 30-second run.
Those receipts bind the stream source checkpoint, not later custody changes.

Before custody production edits, `BenchmarkExistingFileCustody` captured separate
30-second, CPU=1 runs with CPU/memory profiles and retained binaries:
ConfirmDurable 29,954 ns/op, 704 B/op, 11 allocations; Touch 4,860,883 ns/op,
898 B/op, 16 allocations. Both preserve fixed 128-byte content and inode; Touch
also verifies the requested final timestamp. Repeated OS synchronization is the
measured workload. This is a local filesystem result, not a claim about all disks.

`red-custody-namespace` records eight failing rows across Touch and ConfirmDurable
(two accepted confined links and six wrong refusal identities across the other
link forms), plus the parent event. The new 20-row table checks exact inode,
mode, bytes, timestamp and namespace before/after, with empty, read-only,
hard-link, missing and directory controls. Both doors now Lstat the entry,
refuse a non-regular leaf before traversal, and compare it with the opened
file's own observation. Their docs explicitly require callers to coordinate
concurrent namespace edits; name-based Go Chtimes is not an atomic inode stamp.

ConfirmDurable omitted file Sync and only synchronized the parent. Touch's
existing Sync/Close sequence was extracted into `syncCloseCustodyFile` without
changing that sequence; ConfirmDurable now uses it too. The local eight-row
native-handle triad proves successful/empty synchronization, native pipe refusal,
and closure even after Sync fails. The first native test attempt expected
fs.ErrInvalid for a platform errno; its corrected oracle observes the actual Go
pipe Sync errno and retains it. That initial test-oracle failure is not a
production defect. `red-confirm-file-sync-native-oracle` isolates the missing
ConfirmDurable call (one structural row plus parent) while native-handle tests
pass. A structural call obligation cannot prove hardware power-loss behavior.
The docs now promise successful Go/OS synchronization acknowledgment only.

`openRegularReadFile` now returns the FileInfo it already observed beside its
Go file. Custody uses that observation for SameFile; Read and OpenStagedRead
consume it directly instead of taking a duplicate File.Stat. All real internal
call sites were updated, with no compatibility wrapper or exported API change.
`green-custody-sync-and-namespace` passes 261 Go events. Adding the custody semantic
fuzz seeds yields 268 passing events in `custody-final-slice-and-seeds`.

The first ConfirmDurable-bypass mutation survived because the structural test
read the working tree while Go compiled an overlay. That real oracle weakness is
retained in `mutation-custody-confirm-bypass`. The test now uses Go embed to bind
the exact custody source compiled in its build, including overlays. The repaired
control passes, and all four `*-embedded` custody mutations are killed: omitted
file Sync, omitted Close, bypassed ConfirmDurable synchronization, and symlink
following. The last kill occurs inside the new custody fuzz callback itself.
`custody-mutation-analysis.json` records exact edits and selected tests. The
source-embedding correction strengthens the oracle rather than relabeling the
survived mutation as a success.

Full custody package verification, matched after profiles and sustained custody
fuzzing are running next. Platform leaf migration remains separate work. In
particular, Go's Windows Sync calls FlushFileBuffers; native Windows file/directory
access-right behavior must not be assumed from these Darwin receipts. No Windows
runtime success or universal storage durability is claimed.

### Custody checkpoint results

`checkpoint-custody-package` passes 1,087 Go events with no failure/skip and
83.5% statement coverage. The matched 30-second after runs complete with no
source drift. ConfirmDurable moves from 29,954 to 32,252 ns/op, 704 to 944 B/op,
and 11 to 14 allocations. Touch moves from 4,860,883 to 4,868,367 ns/op, 898 to
1,138 B/op, and 16 to 19 allocations. The added no-follow entry observation costs
240 bytes/three allocations; its allocation profile names Go `os.lstatatWithName`.
ConfirmDurable's added synchronization and observation cost about 7.7% in this
single pass; Touch differs by about 0.15%. Neither is claimed as an optimization.
`custody-benchmark-comparison.json` binds samples and profile artifacts.

`custody-sustained-semantic-fuzz` successfully completed its 30-second budget and
reported 440 callbacks. Its progress counter stayed at 440 after the first
three-second update; the report does not extrapolate additional callbacks from
elapsed time. The full transcript is retained. Its seed suite and deliberate
symlink-follow mutation separately establish that the callback reaches both
custody operations and checks actual file contents, inode, mode and timestamp.

## Directory acquisition through Go

The original directory acquisition benchmarks were captured before replacing
platform dependencies: `BenchmarkRootDirectoryAcquisition` measured 36,918 ns/op,
424 B/op and eight allocations; `BenchmarkHeldDirectoryAcquisition` measured
14,889 ns/op, 432 B/op and five allocations. Each is a separate 30-second CPU=1
run, with CPU/memory profiles and the measured binary. Their timed workload
includes acquisition, an actual stat through the capability, SameFile against
the fixture inode, and Close. Native calls dominate both CPU profiles.

Filestore production now imports only Go and its owned Primitive contracts.
`os.OpenFile` owns nonblocking/directory-only acquisition, Stat and Close. Root
construction uses the live descriptor inside `SyscallConn.Control`, then closes
the acquisition file. Held-directory acquisition additionally refuses the final
symlink. Unix flags, stat observations and nonblocking restoration use Go's
`syscall`; Windows handle APIs also use Go's `syscall`, with descriptor access
inside Control. Windows sharing/lock violation errno identities live in Core,
matching Go 1.27.1's own internal Windows definitions (32/33). No dependency was
removed from go.mod because other packages still use it. Filestore's last
external syscall test import was also replaced by the standard library.

The architecture import test embeds this build's Go sources, so it checks the
actual compiled mutation rather than a separate working-tree program. Its
initial red run catches all five external platform imports. Existing compiler
shape and data-flow checks remain. The nonblocking preparation matcher now
counts calls rather than method-value selectors, and six synthetic source rows
pressure false admission, direct descriptor use, mixed calls and unrelated
owners. Its function and method names are derived from compiler-bound types.
The named-pipe acquisition process backstop now uses Temporal.

The local descriptor table has eight rows covering independent file/root
ownership, renamed/replaced paths, empty contents, and native nil/closed/regular
file refusal. The public held-directory triad replaces three isolated examples
with eight rows checking context/path admission, native refusal, original inode,
children, release of borrowed handles, and close-error custody. The first local
attempt incorrectly expected fs.ErrClosed from RawConn.Control; the corrected
oracle obtains Go's actual native raw-connection identity and compares it with
errors.Is. This is retained as a test-oracle failure, not a production defect.
The corrected combined slice has 27 passing Go test events, no failure or skip.

`FuzzDirectoryAcquisitionNamespaceCustody` exercises both public acquisition
doors, link acceptance/refusal, real namespace renames/replacements, fuzzed child
names and binary contents, exact inode observations and absence of extra files.
Its first seed run exposed a test assumption: Go ReadDir's lazy DirEntry.Info
uses the old pathname outside an os.Root. Go Readdir obtains the FileInfo
through the held descriptor. The callback now uses Readdir, and the borrowed
File documentation names this standard-library distinction. No replacement
runtime or directory implementation was added. All six corrected seeds pass.

Four intentional semantic mutations are killed without build failures: reopen
the old name instead of the descriptor; follow a final directory link; reintroduce
an external syscall dependency; and hide an owned Close error. The link-follow
mutation is killed inside the fuzz callback. `directory-mutation-analysis.json`
binds the changes and receipts; all four runs have no source drift.

The Linux amd64 and Windows amd64 package test binaries compile successfully
with the effective GOOS/GOARCH captured. These are compile-only receipts, not
runtime filesystem claims. Native full-package verification, matched directory
after benchmarks and sustained directory fuzzing remain to be recorded below.

### Directory checkpoint measurements

`checkpoint-directory-platform-package` passes 1,251 Go test events, no failure
or skip, with 84.1% statement coverage. Counts include parent and seed events.
Both after measurements complete with no source drift: root acquisition is
38,610 ns/op, 528 B/op, 10 allocations versus 36,918/424/8 before; held-directory
acquisition is 14,748 ns/op, 656 B/op, six allocations versus 14,889/432/five
before. Root is about 4.6% slower in this single pass; held-directory timing
differs by about -0.9%, which is not a demonstrated speedup. Go-owned acquisition
adds 104 bytes/two allocations to root construction; Go File.Stat adds the
224-byte/one-allocation held-directory observation. Native execution still
dominates CPU samples. These are explicit standard-library ownership costs,
not an optimization claim. `directory-benchmark-comparison.json` binds the
samples and retained CPU/memory profile and binary artifacts.

The directory fuzz run passed its 30-second budget and reported 2,608 executions,
then a plateau. A separate debug run (`directory-fuzz-accounting-diagnostic`)
showed Go waiting on minimization of coverage-increasing inputs; it reported
850 executions, and the pending minimization only returned when the overall
budget expired. The native sample caught the tail of worker shutdown, lacks Go
symbols, and is not treated as CPU benchmark evidence. A further unchanged-source
30-second run sets Go's `-fuzzminimizetime=1s`: progress continues throughout and
26,728 executions are reported. This is a bounded minimization budget, not a
weaker semantic oracle or disabled minimization. All three receipts remain.

## Inspection fact validation continuation

The bounded-minimization custody run reports 8,354 executions over 30 seconds,
with progress throughout. The first attempt did not run fuzzing: a newly added
inspection benchmark used Temporal's two-result Nanoseconds method as one
value, and failed to build. The compiler error is retained; the corrected run
is `custody-sustained-bounded-minimization-corrected`.

Inspection before measurements use the Go-owned directory implementation above.
Real-file inspection with checked kind/extent/time/permissions is 40,642 ns/op,
856 B/op, 12 allocations. The initial single-validation benchmark hit Go's
billion-iteration cap at 2.616 ns/op, before its requested 30 seconds. That run
is retained as exploratory and is not duration-compliant acceptance evidence.
Its replacement has a distinct identity, `BenchmarkInspectionValidationBatch`,
and performs exactly 64 validations per operation: 189.7 ns/op, zero allocations,
with a completed 30-second budget. Both retained workloads have CPU and memory
profiles and their measured binaries. The zero-allocation validation memory
profile measures profiling/runtime setup, not hot-path allocations.

`red-inspection-schema-closure` records 13 failed schema rows plus parent. These
are private typed-fact contradictions accepted by the kind-only validator, not
13 OS incidents. The 23-row local table pressures absent metadata, unset versus
observed zero facts, permissions carrying kind bits, UID/GID without ownership,
allocation without reporting, and regular-file facts attached to a directory.
Accessors must refuse the entire invalid observation and leak no partial facts.
Inspection now validates these combinations. Unix allocation observation ignores
non-regular entries, matching the public storage contract. The allocation block
unit is now the compiler-owned `core.POSIXAllocationBlockBytes`, shared by the
production projection and boundary fixtures.

A separate kind-only constructor table covers all six admitted kinds and two
invalid kinds. Its initial run exposes five nonzero error results plus parent;
construction now returns zero on refusal. Existing native inspection/attribute
checks and the schema table pass 109 events. Adding the constructor cases and
new native fuzz seeds produces a focused 58-event pass. These counts describe
separate selections, not additive totals or an earned-row quota.

`FuzzInspectionNativeFactsSemanticClosure` creates real regular files, directories,
symlinks, named pipes, absent names and unreachable parents. It varies binary
contents and permission bits, then checks exact kind, timestamp, mode, ownership,
regular-file extent/allocation, no leaked absent facts, retained inode/content,
and unchanged namespace. The fixture uses native Go handles, including a held
read handle for a file whose permissions were removed. Seed selectors bind to
the typed PathKind inventory. Inspection docs now describe only mechanical
observations; unreported allocation never implies caller acceptance.

### Inspection schema checkpoint

All nine inspection mutations are killed without build failure, including
independent UID/GID corruption, getter bypass, nonzero constructor errors,
final-link following, unreachable/absent conflation, and allocation scaling.
`inspection-mutation-analysis.json` binds each edit. Full package verification
passes 1,293 Go events with no failure or skip and 84.6% statement coverage.
The native inspection fuzz target completes 30 seconds with 58,112 executions.

The matched schema after runs are 41,413 ns/op, 856 B/op, 12 allocations for
real-file inspection versus 40,642/856/12 before. The 64-validation batch is
660.3 ns/op versus 189.7 ns/op, still zero allocations: about 10.3 ns per full
validation versus 3.0 ns for the previous kind-only check, including loop work.
Real-file timing differs by about +1.9% in this pair; that is not a measured
speedup. CPU profiles attribute the added pure-validation work to the fact
validators, while native calls still dominate the real-file observation.

### Profile-informed direct Lstat continuation

About 87% of real-file inspection CPU was spent acquiring a parent root before
observing one absolute path. Direct Go Lstat already provides the needed final
no-follow observation and never opens a FIFO. Two additional unchanged-source
30-second before samples are 40,537 and 40,301 ns/op, both 856 B/op and 12
allocations; together with 41,413 they form the before distribution for this
separate simplification. Their shared CPU/memory profiles and binary are retained
in `inspection-before-direct-lstat-distribution`.

New native rows confirm two defects in the former implementation: filesystem
roots were rejected for having no final component, and searchable parents without
listing permission were refused. `red-inspection-direct-go-boundaries` has five
failed rows (one root and four search-only permission cases) plus two parent
events. The owner-permission table exhausts all eight combinations with both
present and absent children, pins the native Go Lstat result first, and checks
unchanged namespace and content. Its permission-refusal branch requires a
non-root Unix execution; this Darwin receipt ran without skips.

Inspect now delegates an existing path directly to os.Lstat. Only a missing path
requires parent Stat to distinguish reachable absence from unreachable storage.
No parent handle, descriptor adapter, or copied path component is needed for
that observation. The entry-construction helper now accepts the successful Go
FileInfo directly; its old error-bearing signature was removed and the real
caller updated. No exported API or compatibility layer was added.

The former kind-only example table was replaced with exact native metadata and
custody checks, including final and intermediate links. Six additional Unix
sparse-file rows cross both 32-bit conversion boundaries without loading their
extents into memory. They are confined to Unix, since Windows needs explicit
sparse-file control to make the same bounded-disk fixture promise. Other native
observation cases remain cross-platform, with an explicit unsupported-FIFO skip
only where that native capability is unavailable. The first focused direct-Go
slice passes 305 events with no failure or skip. Final direct-Go checkpoint and
after distribution remain to be recorded.


### Direct Lstat checkpoint

The final direct-Go package check passes 1,306 Go test events, zero failures or
skips, with 84.7% statement coverage. Sustained inspection fuzzing completes
30 seconds and 60,987 executions. Linux amd64 and Windows amd64 test binaries
compile; those are not runtime or native-permission proofs.

All six direct-Go semantic mutations are killed without compiler failure,
including final-link following, absent/unreachable conflation, componentless
root refusal, unnecessary parent listing permission, and signed/unsigned extent
narrowing. The exact replacements and outcomes are retained in
`inspection-direct-mutation-analysis.json`.

Three 30-second after samples are 2,200, 2,197 and 2,196 ns/op, all 320 B/op and
2 allocations. The before distribution is 41,413, 40,537 and 40,301 ns/op, all
856 B/op and 12 allocations. Medians are 40,537 and 2,197 ns/op (18.45 times
less elapsed work for this fixed existing-file observation). This does not
claim the same ratio for missing paths or other filesystem operations.
`inspection-direct-benchmark-comparison.json` retains all samples and binds
source/run receipts and profile artifacts. After CPU is 99.66% native syscall;
allocation samples belong to Go's Lstat FileInfo and path conversion. No custom
filesystem cache or competing syscall implementation was introduced.


## Caller stream unwinding

`red-stage-unwind-custody` fails four panic rows plus parent: a caller Read
panic left the exact owned Go file open and its temporary namespace unsettled.
`red-read-unwind-custody` fails two panic rows plus parent: a caller Write
panic left the source handle open. That red Read run follows a behavior-preserving
extraction of the owned-file transfer into `readOwnedRegularFile`, allowing
its actual handle to be checked directly without process-global descriptor
counts or eventual finalization.

Stage now defers abandonment while the caller's copy has not returned. Read
owns one deferred close that joins ordinary return errors. Both retain Go panic
propagation; there is no panic-to-error translation, custom runtime, or fabricated
byte acknowledgment. Panic/Goexit cleanup has no normal error return channel:
Stage attempts identity-checked abandonment without replacing the active unwind.
It does not promise cleanup success against caller namespace sabotage or OS
cleanup refusal. The native table retains the original inode under an archive
name while installing a foreign replacement, proving cleanup closes its handle
without removing the replacement or confusing inode reuse with ownership.

The local tables check empty completion, exact binary ceilings, returned native
failure, panic before an effect, after a completed prefix, at the overflow probe,
and after a destination's physical write. The public semantic fuzz target reaches
Stage, Write and Read and checks ordinary versus unwound receipts, exact disk and
writer effects, refusal cleanup, original target identity and neighboring bytes.
The focused control passes 25 events. All five mutations are killed without
build failure: skipped abandonment, missing owned close, foreign-inode deletion,
missing read close and discarded native error. Their exact overlays are in
`stream-unwind-mutation-analysis.json`.

The full checkpoint passes 1,331 Go events, zero failures or skips, with 85.0%
statement coverage. Sustained public unwind fuzzing completes 30 seconds and
5,598 executions. The matched 128-byte stage/read/discard lifecycle is
11,459,325 ns/op before and 11,157,228 after, both 2,301 B/op and 36 allocations.
Public Read is 15,880 before and 16,120 after, both 688 B/op and 11 allocations.
Each run requests 30 seconds, CPU1, CPU/memory profiles and a retained binary.
These single pairs establish unchanged allocation counts, not timing trends.
Profiles remain dominated by native file operations; short lifecycle allocation
profiles also include substantial runtime/profiler startup samples.
`stream-unwind-benchmark-comparison.json` binds all four runs and artifacts.

## Concurrency and time ownership continuation

Three overlapping concurrency implementations were consolidated into one five-row
`TestNamespaceConcurrencyLayerTriad`. It retains 10,000 distinct writers and 64
exclusive contenders, releases them through a ready/start barrier, joins every
owned worker, and checks one indexed result per request, settled recovery
receipts, exact binary payloads, modes, extents and namespace cardinality. The
barrier does not claim simultaneous kernel execution or a latency bound.

The former neutral row passed an empty slice to a test helper and never called
production. Its replacements execute real empty-source writes and contend
against an already-existing empty target; another row submits canceled calls
and proves no new entries. A separate neighbor remains byte-for-byte intact.
The focused concurrency/FIFO/deadline/stream-cancellation run passes 20 Go events,
zero failures/skips, including the full 10,000-writer row. Three targeted mutations
are killed without build failure: overwrite an existing create-only target,
invent empty-write success without an effect, and return a recovery request after
completed activation. Exact overlays are in
`namespace-concurrency-mutation-analysis.json`.

The remaining direct `context.WithTimeout` and `time.After` effects in Filestore
tests now use Temporal. A setup-only `newFilesystemBackstop` constructs typed
Temporal requests; each test retains its own completion/effect assertions and
worker joins. Timeout budgets are unchanged. This source cleanup is not a
repository gate or a timing benchmark.

## Removal continuation — checkpoint running

The prior RemoveTree tests were two ad hoc examples. The replacement ten-row
native table pressures binary files, empty directories, nested trees with
outside links, final and dangling links, reachable absence, missing parents,
root naming, canceled execution and closed rooted capabilities. It runs Go
Root.RemoveAll against an independent fixture where that operation is admitted,
then checks native errors and exact retained namespace, neighboring bytes and
outside inode/metadata. The first attempt had a misspelled PathKind enum and
failed to build; it is retained separately from behavioral evidence.

The corrected red run fails one row plus parent: Go treats a path below a missing
parent as absent, while Filestore reported a cleanup failure by synchronizing a
parent that never existed. RemoveTree now observes Lstat before effect, returning
quietly on genuine absence and preserving dangling links as occupied entries.
Existing entries still go through Go Root.RemoveAll and parent synchronization.
Concurrent unsynchronized namespace replacement is not an atomic multi-operation
contract; the wrapper does not invent one.

The new fuzz target compares real public Remove and RemoveTree results with Go
on independent bounded fixtures. It varies binary contents, entry kinds, child
counts, leaf versus recursive removal and cancellation, then independently walks
both resulting fixtures and compares every retained entry, mode, link target and
byte. The supplementary compiled-source guard retains parent-sync calls; its
synthetic rows distinguish a call from a function value or another owner's call.
That guard is not proof of physical power-loss durability or arbitrary control
flow. Focused namespace, source-guard and fuzz-seed verification passes 33 Go
events with no failure or skip.

The before empty-directory lifecycle benchmark measures Go Mkdir followed by
Filestore durable RemoveTree: 5,669,840 ns/op, 1,562 B/op and 16 allocations, with
30 seconds, CPU1, CPU/memory profiles and retained binary. Exclusive Mkdir on
every iteration and final namespace observation reject a no-op removal workload.
Final mutation, full-package, after benchmark and sustained fuzz results remain
to be recorded.


### Removal checkpoint

All six removal mutations are killed without compiler failure, including omitted
parent synchronization. The runner initially miscounted synchronization sites
because prepareCreatedAppend also calls syncParent; it stopped after five
completed semantic kills. Resume verified unchanged source digests and exact
saved overlays before reusing those records. This orchestration correction is
retained in `removal-checkpoint-orchestration-repair.json`; it is not a production
regression or a hidden rerun.

Full package verification passes 1,362 Go events, no failures or skips, with
85.2% statement coverage. Sustained removal fuzzing completes 30 seconds and
7,359 executions. The after lifecycle measurement is 5,511,045 ns/op, 1,794 B/op,
19 allocations versus 5,669,840 ns/op, 1,562 B/op, 16 allocations before. The
extra Lstat costs 232 B and three allocations for this existing directory.
The lower single timing sample is not a performance trend. CPU remains dominated
by native filesystem operations; the added allocation samples appear in Go
lstat metadata/path handling. Both 30-second CPU1 runs retain CPU/memory profiles
and measured binaries, bound in `removal-benchmark-comparison.json`.


## Permission checkpoint

SetPermissions previously changed an inode's mode and synchronized only its
parent directory. The compiled-source red check establishes that missing
file-sync call; it does not simulate physical power loss. The implementation
now uses Go rooted nonblocking acquisition, File.Chmod, File.Sync and File.Close
on the same held entry. Read access is preferred; permission refusal tries
write access without truncation and retains both native errors if refused.
The namespace is unchanged, so parent synchronization is removed.

This introduces a material capability requirement: an entry must permit native
read or write acquisition before its mode changes. Execute-only files and
search-only directories can therefore refuse before effect. Write-only ingress
is supported. After a successful Chmod, a sync/close failure returns the typed
indeterminate identity and native cause. No temporary permission widening,
custom filesystem execution or product policy is introduced.

The native table pressures exact modes, write-only fallback, unchanged modes,
directories, confined and escaping links, missing entries, canceled and closed
capabilities, and forbidden request modes. Separate Unix rows prove refused
acquisition leaves inode, metadata, data and namespace unchanged. The fuzz
oracle compares Go Chmod on an independent fixture, then checks exact bytes,
mode, identity, link retention and neighboring namespace for files, directories,
links and cancellation.

An initial test assumption that a named FIFO cannot synchronize was wrong:
Go File.Sync succeeds on that native Darwin handle. That control failure was
caught and retained. The provisional kind gate and redundant Stat were removed;
Go now decides synchronization support. A mistakenly reported mutation kill
from the failed fixture is explicitly invalidated in the old analysis, and the
old full-package run was interrupted and is not accepted evidence. Details are
in permission-fifo-oracle-correction.json. The corrected FIFO test uses a real
native synchronization oracle, and all seven corrected mutations are killed
without build failure in permission-corrected-mutation-analysis.json.

The corrected package run passes 1,390 Go events, zero failures/skips, with
85.0% statement coverage. Linux and Windows amd64 test binaries compile; native
execution here is Darwin arm64. The corrected sustained fuzz run completes
30 seconds and 5,448 executions.

Two observed mode changes measure 9,390,134 ns/op, 1,269 B/op and 24 allocations
before versus 9,447,149 ns/op, 948 B/op and 16 allocations after. The 0.61% timing
difference in one pair is not a trend. The lower allocation count follows
removal of parent-root work. CPU samples remain almost entirely Go/native
filesystem execution. Allocation profiles include profiling startup at these
operation counts. Both 30-second CPU1 runs retain CPU/memory profiles and exact
measured binaries, bound in permission-benchmark-comparison.json.


## Direct-writer and recovery handoff checkpoint

The three ad hoc direct-writer examples are replaced by a 14-row native
custody-name table. Finish and abandon distinguish an owned entry, missing
name, same-byte foreign inode, foreign directory with child, symlink resolving
to the original, and a real hard link. Empty completion and empty abandonment
remain real effects. Every row checks the settled Go handle, exact zero or
validated receipt, retained inode/metadata and full bounded namespace contents.

The linear-ownership table now has eight earned rows covering nil, zero,
copied-live, copied-before-finish, finished and abandoned handles, plus canceled
finish calls that must not seize the original or transferred custody. Ingress
has 14 named refusals with exact native causes and preserved files/directories,
including dangling/outside links and a closed rooted capability. The extent
matrix retains 12 distinct cases; polite repeated byte counts and purported
32-KiB direct-file buffering boundaries are removed. A seven-row post-finish
matrix independently mutates extent, mode or namespace before real Commit,
checking refusal preserves the exact fixture and cleanup respects ownership.

The direct-writer fuzz target varies bounded binary payload, declared extent,
fragmentation, copy attempts, cancellation, foreign replacement and abandonment.
It follows real receipts through staged read and Commit. The recovery fuzz
obtains a nonzero CommitRequest from an actual interrupted Write; it preserves
the original inode with a hard link, proves foreign Recover/Discard refusals,
restores custody when selected, pressures create-only conflicts versus replace,
and checks completed recovery is idempotent with exact bytes and metadata.
Neither target invents a recovery request for a settled Write.

All 13 final mutation cases are killed without build failure. An initial
single-site oversized-extent mutation survived because a later receipt check
still rejected the wrong extent. That survival is retained in
stage-destination-mutation-defense-redundancy.json, not counted as a kill.
The final over/under-extent cases remove both redundant defenses and the public
fuzz oracle rejects the wrongly accepted receipt. Other kills cover copies,
mode drift, missing file/parent synchronization, leaked handles, symlink
following, foreign cleanup, erased Write handoff, lost recovery idempotence and
create-only recovery overwrites. The source synchronization guard is explicitly
supplementary; its hostile matcher rows reject borrowed and uncalled methods.

This slice changes tests, fuzz and benchmarks; it does not claim new production
regressions. Focused final controls pass 89 Go events. The full package passes
1,433 events, zero failures/skips, with 85.2% statement coverage. Each new fuzz
target completes its 30-second budget: 6,418 direct-writer executions and 2,091
recovery-handoff executions.

The new fixed 128-byte direct-writer/create-activation/read-verification/removal
lifecycle measures 22,440,214 ns/op, 3,179 B/op and 51 allocations. It is an
initial measurement of a new workload, not a before/after improvement claim.
The measured invocation is 30 seconds, CPU1, with CPU/memory profiles and the
exact binary retained in stage-destination-benchmark-record.json. Native syscall
and fcntl frames account for 99.83% of sampled CPU. Memory samples primarily
show Go path/stat/file handling, plus profiling startup at this operation count.


## Native handle checkpoint

Read and update now share a 32-row native capability table. It pressures
access flags, binary and empty files, confined/escaping/dangling links,
directories, invalid and closed requests, and a held inode after its name is
replaced. Append has 22 rows covering all three intents, exclusivity, dangling
fallback refusal, append-after-seek, preserved existing permissions and exact
prefix bytes. Rotation has 21 ownership rows covering validation before
consumption, native synchronization failure, a syncable native directory,
occupied incoming names, and exact outgoing closure and incoming creation.
Weak overlapping read/update/append/rotation examples are retired.

Lock acquisition has 18 rows covering exact mode changes, non-truncation,
dangling-link creation, strongest native refusals and caller read/write access.
Five real filelock acquisition rows distinguish same inode by name or hard
link from a different inode with identical bytes; release enables a previously
contending acquisition. Native regularity tables check refused handles close
without closing an independent peer. Four FIFO rows prove public read, update,
append and lock acquisition refuse a nonregular handle without consuming the
live peer's queued bytes. Assertions inspect native handles, full bounded
namespace contents, bytes, modes, identities and retained timestamps directly.

Two semantic fuzz targets exercise the four acquisition doors and append
rotation, including binary payloads, caller writes, foreign name replacement,
closed/nil capabilities, cancellation and every append intent representation.
All 19 final mutations are killed without build failure. An initial mutation
that made a local variable unused was a compiler failure, not a kill; the
corrected mutation and the source-verified reuse of four prior results are
recorded in native-handle-mutation-orchestration-repair.json. Two initial test
compilation mistakes are also retained, not called production regressions.
This slice changes tests, fuzz and benchmarks; production is unchanged.

The package checkpoint passes 1,531 Go events, zero failures/skips, with 86.2%
statement coverage. Linux and Windows amd64 test binaries compile; execution
here is Darwin arm64. Both sustained fuzz targets complete 30 seconds:
15,668 acquisition executions and 3,161 rotation executions.

Five new fixed workloads each ran separately for 30 seconds on CPU1, with
CPU/memory profiles and the exact measured binary retained. Acquisition plus
native observation/close measures 15,430 ns/op, 653 B/op, 7 allocations for
Read; 23,053 ns/op, 653 B/op, 7 allocations for Update; 16,169 ns/op, 669 B/op,
7 allocations for AppendExisting; and 35,683 ns/op, 653 B/op, 7 allocations for
Lock. The outgoing-rotation/incoming-close/durable-removal lifecycle measures
10,447,563 ns/op, 1,852 B/op and 27 allocations. These are initial measurements,
not before/after optimization claims or payload-throughput measurements.
Native system calls dominate CPU; lock Fchmod alone is 33.28% cumulative.
Allocation samples primarily show Go Stat, path and file ownership, with
profiling startup visible in the slower rotation workload. Exact records and
artifact digests are bound in native-handle-benchmark-records.json.


## EnsureDirectory checkpoint

EnsureDirectory now acquires an already existing final directory once through
Go rooted open, then observes, changes permissions, synchronizes and closes
that same held Go file. Only a missing acquisition enters directory-chain
creation. A later mode/sync/close failure does not retry creation. Existing
ancestors keep their permissions; each newly created name retains its existing
parent-sync requirement. No custom path resolver or OS execution layer is
introduced. Go's rooted traversal still owns ancestor capability requirements.
The documentation states that failure may leave an exact partial created prefix.

The 28-row native namespace table pressures exact new/existing permissions,
umask restoration, retained child bytes, confined/dangling/escaping/looping
links, file obstacles, invalid/canceled/closed capabilities and a root whose
external name is replaced. A separate 24-row matrix exhausts all eight owner
permission combinations for existing-final, new-final and new-chain effects.
It pins real native acquisition/extension refusal and exact partial-prefix
state without widening permissions to make an operation succeed. Eight native
held-file rows prove exact mode changes, untouched regular-file refusals,
closure and retained peers. Compiled-source synchronization guards supplement
native observations; they do not simulate physical power loss.

The semantic fuzz target varies bounded depth, existing prefix, final entry
kind, binary children, mode and cancellation. Its namespace/mode oracle uses
Go MkdirAll and native observations on an independent bounded fixture, including
outside namespace, inode and metadata preservation. Existing and missing
cancellation seeds reach both acquisition and creation paths.

An initial native table control incorrectly compared Go OpenRoot's fresh
regular-file error with O_DIRECTORY's syscall error. That is a test-oracle
mismatch, retained in ensure-directory-native-oracle-correction.json, not a
production regression. An initial cancellation mutation survived a selected
missing-target row because chain creation has another cancellation check.
The missing existing-target test and fuzz seed were added, and all 13 final
mutations were rerun and killed. The survival and initial four historical kills
are retained in ensure-directory-cancellation-mutation-gap.json; no build
failure is counted as a kill in the final analysis.

The package checkpoint passes 1,612 Go events, zero failures/skips,
with 87.0% statement coverage. Linux and Windows amd64 test binaries compile;
execution is Darwin arm64. Sustained directory fuzzing completes 30 seconds
and 6,341 executions.

All six benchmark invocations retain CPU/memory profiles and exact binaries
in ensure-directory-benchmark-comparison.json. Each is 30 seconds on CPU1.

- Depth1: 9,398,298 → 9,592,481 ns/op; 1,780 → 1,604 B/op; 24 → 16 allocations.
- Depth16: 16,099,612 → 11,762,199 ns/op; 51,575 → 3,669 B/op; 1000 → 54 allocations.
- Create: 11,492,946 → 12,272,313 ns/op; 6,134 → 6,262 B/op; 65 → 69 allocations.

These are single before/after pairs, not timing distributions. The baseline
Depth16 profile attributes 44.52% cumulative CPU to repeated existing-ancestor
validation; 39.30% of allocation bytes appear in repeated Go rooted path
splitting. The change removes those repeated ancestor-level operations for an
existing final directory. Creation still pays native synchronization and now
also pays an initial missing acquisition; its cost is reported separately.


## Rename checkpoint

The old presence-only Rename matrices and their assertion-hiding helpers are
removed. Their OpenParent fixture call sites now use direct Go fixture setup;
OpenParent itself remains a later slice. Rename has a 39-row native table that
checks binary/empty content, inode custody, held old-target bytes, all relevant
file/directory/link replacements and refusals, hard-link no-ops, aliases of
one physical parent, escaping ancestors, invalid/canceled/closed capabilities
and exact native LinkError causes. Whole bounded namespaces are compared with
an independently built Go Root.Rename fixture; outside bytes, inode, mode and
time remain unchanged.

Eight owner-permission rows pin real native pre-effect refusal versus
post-effect parent-sync refusal. A writable/searchable directory without read
permission permits rename through the held root but refuses parent acquisition
for synchronization. Filestore must return the indeterminate identity while
retaining the completed move. A failure before rename keeps both original
entries and must not claim an indeterminate effect. The compiled-source guard
retains the exact new-parent and old-parent sync arguments; it supplements
native failure observations and makes no physical power-loss claim.

The fuzz target varies bounded binary source/target payloads, native entry
kinds, parent layouts, hard links, cancellation, closed roots and identical
request paths. Hard-link fixtures prove real inode equality before execution.
It distinguishes a no-op through two aliases of one regular entry from Go's
refusal of the corresponding directory basename. The oracle checks both wrong
acceptance and wrong refusal, exact namespace state and retained source/target
identity, plus unchanged outside material.

An initial table control expected OS-style replacement of an empty directory.
Go Root.Rename deliberately follows os.Rename's stricter existing-directory
rule; the native control caught that mistaken expectation. The failure and
correction are retained in rename-native-directory-oracle-correction.json.
The production documentation now states Go's replacement and no-op rules.
Executable Rename behavior is unchanged; this slice does not claim a new
production regression. All 13 mutation cases are killed without build failure,
including omitted/misrouted sync, invented linking, hard-link no-op deletion,
ignored cancellation, wrong error phase and rollback of an indeterminate move.

The full package checkpoint passes 1,677 Go events, zero failures/skips,
with 87.3% statement coverage. Linux and Windows amd64 test binaries compile;
execution is Darwin arm64. Sustained Rename fuzzing completes 30 seconds
and 5,334 executions.

Two new fixed binary-inode round-trip workloads each ran separately for
30 seconds on CPU1 with CPU/memory profiles and exact measured binaries:

- BenchmarkRenameBinaryInodeRoundTrip/SameParent: 10,292,539 ns/op, 4,197 B/op, 48 allocations.
- BenchmarkRenameBinaryInodeRoundTrip/CrossParent: 20,956,819 ns/op, 4,858 B/op, 66 allocations.

These are initial workload measurements, not a before/after performance claim.
Native syscall/fcntl frames account for 99.37% of sampled same-parent CPU and
98.98% for cross-parent CPU. Allocation samples include Go metadata/path
handling, the declared independent byte observations and profile startup.
Exact records and artifact digests are bound in rename-benchmark-records.json.


## Parent acquisition and root identity checkpoint

OpenParent now has 14 earned native capability rows, plus eight Unix owner
permission rows. The old leaf-kind permutations were removed because this door
acquires the parent without inspecting the leaf. The table proves exact returned
basename and held parent inode, missing children, aliases, parent replacement,
zero/canceled requests and native acquisition refusals. Namespace bytes, links,
inodes, modes and modification times remain unchanged by observation.

ValidateRootIdentity has 13 native rows covering retained roots, aliases,
renaming, foreign same-content replacement, final symlinks, nil/closed handles
and invalid/unreachable paths. Identity is Go SameFile, never diagnostic names,
file sizes or path spelling. Contract refusals explicitly exclude Source;
otherwise the shared error ancestry could hide a wrong classification.

The combined semantic fuzz target begins with a real OpenParent and successful
identity validation, then varies native parent shape, child representation,
payload, cancellation and capability/name replacement. Exact native inode facts
and bounded namespace observations distinguish acceptance and refusal. The
30-second campaign completed 11,171 executions. All 13 final semantic mutants
were killed. The first child-acquisition mutation failed to compile because
it left a local unused; that attempt is retained and excluded from kills in
parent-identity-mutation-compile-correction.json.

The full package checkpoint passed 1,727 Go events with zero failures/skips
and 88.4% statement coverage. Linux and Windows amd64 test binaries compiled;
runtime evidence is Darwin arm64. Production executable behavior was unchanged
in this slice. Two initial workloads ran separately for 30 seconds on CPU1
with CPU/memory profiles and retained binaries:

- BenchmarkOpenParentNativeIdentityCustody: 45,495 ns/op, 1,304 B/op, 18 allocations.
- BenchmarkRootIdentityOwnedAndForeignDirectories: 7,236 ns/op, 1,196 B/op, 13 allocations.

These are workload baselines, not an optimization comparison. The parent
profile attributes 86.88% cumulative CPU to native root acquisition; the
identity-pair profile attributes 99.42% flat CPU to native syscall execution.
Allocation profiles include Go metadata/path work and the declared independent
observations. Exact commands, sample counts and artifact digests are bound in
parent-identity-benchmark-records.json.


## Symbolic-link observation and canonical resolution checkpoint

ReadSymbolicLink has 20 native observation rows; Canonicalize replaces five
ad hoc tests with a 16-row Go-resolution table. These pin opposite semantics:
ReadSymbolicLink preserves the first opaque target without following the final
link, whereas Canonicalize delegates full resolution to filepath.EvalSymlinks.
Dangling links, cycles, confined and escaping ancestors, absolute targets,
lexical target bytes, wrong entry kinds, closed roots, cancellation and zero
requests cannot silently cross that distinction. Exact namespace bytes, link
targets, inodes, modes and modification times survive both observations.

Eight Unix owner-permission rows prove native search capability without
inventing a read/write permission prerequisite. Source failures retain native
PathError fields and errno identities; contract refusals explicitly exclude
Source. Go's link-count exhaustion is a fresh untyped error, so its Source
wrapper is the stable identity; that upstream gap has a local waiver.

The nominal SymbolicLinkTarget table pressures empty, one-byte, byte-ceiling
minus one/exact/plus one, embedded NUL, invalid UTF-8 and multibyte overflow.
Targets are opaque bytes, never a second path language. A separate three-row
native admission table distinguishes OS materialization from typed admission.
The initial fuzz oracle incorrectly assumed native Readlink success implied
nominal admission: Darwin can materialize an empty target. Production correctly
refused it. The failed run and correction remain in
symbolic-observation-native-empty-oracle-correction.json; no production bug is
claimed for that correction.

Both fuzz targets begin with actual ReadSymbolicLink output; the combined
external target also builds a successful canonical path through production.
It varies native target bytes, entry shapes, outside ancestors, contexts and
root custody, then independently checks Go execution, nominal admission and
unchanged bounded namespaces. The direct nominal target reaches the full
64 KiB ceiling separately from smaller native filesystem link limits.
Thirty-second campaigns completed 6,408 native-boundary executions and 319,153
nominal executions. All 18 semantic mutations were killed without build failures,
including root escape, premature following, truncation, omitted byte bounds,
wrong error identities and partial results on refusal.

The full package checkpoint passed 1,805 Go events, zero failures/skips,
with 88.7% statement coverage. Linux and Windows amd64 test binaries compiled;
runtime evidence remains Darwin arm64. Canonicalize documentation now states
Go resolution and its observational limits, without product scenarios or a
claim of filesystem case normalization. Executable behavior was unchanged.
Two initial benchmark workloads ran separately for 30 seconds on CPU1 with
CPU/memory profiles and retained binaries:

- BenchmarkReadSymbolicLinkExactOpaqueTarget: 1,453 ns/op, 160 B/op, 4 allocations.
- BenchmarkCanonicalizeAncestorAndFinalLink: 33,769 ns/op, 6,256 B/op, 61 allocations.

These are new workload baselines, not a speedup claim. Native syscall frames
account for 99.06% and 97.98% of sampled CPU respectively. Allocations reside
in Go's readlink buffers, root path handling, metadata and symlink traversal.
Exact commands, sample counts and artifact digests are bound in
symbolic-observation-benchmark-records.json.


## Native pipe custody checkpoint

Production remains a direct os.Pipe call after context admission. Twelve
custody rows prove exact binary/empty/512-byte streams; nil, panicking typed-nil,
nil-safe, canceled and expired contexts; cancellation after transfer; and
native closed-end failures, including zero-byte writes. Nine admission rows
prove missing/aliased custody, reversed shape, independently closed endpoints
and unchanged buffered bytes. Validate does not inspect live descriptor state
or consume data. The Temporal-backed test backstop closes only owned handles
and joins its cancellation callback before cleanup.

An owned subprocess lowers only its own descriptor limit. Its zero/one/two
available-slot cases compare with native os.Pipe, requiring exact EMFILE and
SyscallError preservation, zero refused endpoints and usable two-slot custody.
Refilling the same available slots detects a descriptor leaked behind a zero
public result. The successful pair transfers exact binary bytes and EOF and
returns both slots on caller Close. Child selection uses the actual compiled
test symbol. These three child cases are retained in the transcript, not added
to the parent's Go event count.

The fuzz target starts with bytes emitted through a real admitted pipe and
varies bounded payloads, fragment sizes, contexts and endpoint closure. It
checks exact bytes, native refused write causes, EOF, zero refused custody and
structural admission after closure. Thirty seconds completed 255,803 executions.
All 14 semantic mutations were killed without build failure, including erased
resource failure, swapped/closed endpoints, missing custody gates and validation
that closes or consumes a handle.

An initial fuzz oracle assumed zero-byte writes bypassed broken-pipe refusal.
Darwin disproved that assumption; the independent Go control now receives the
actual fragment. pipe-zero-write-oracle-correction.json retains the failed
attempt. Production was unchanged.

The first package checkpoint passed 1,831 Go events, but the raw child transcript
triggered cmd/go's first-coverage-line extraction: its summary printed the
child's 0.6% while the parent artifact recorded 88.7%. The complete transcript
is now retained losslessly as base64 in the test log. A fresh package run again
passes 1,831 events, zero failures/skips, and command/artifact both report 88.5%.
The two-statement difference is pre-read context refusal in streamReader; the
remaining sweep must pin that handoff deterministically. Exact accounting and
the decoded child transcript are in pipe-child-coverage-accounting-correction.json.
Its report reader reassembles split Go JSON output events before base64 decode.
Linux and Windows amd64 test binaries compiled before this diagnostic-only
logging change; final compilation remains pending with other sweep checks.

Two initial workloads ran separately for 30 seconds on CPU1 with CPU/memory
profiles and measured binaries before that diagnostic-only logging change:

- BenchmarkPipeAcquireBinaryEOFAndClose: 8,243 ns/op, 208 B/op, 4 allocations.
- BenchmarkPipeHeldBinaryTransfer: 1,281 ns/op, 0 B/op, 0 allocations, 399.84 MB/s.

These are new workload baselines, not an optimization claim. Acquisition CPU is
71.24% native syscall and 28.19% Go kevent; 99.49% of sampled allocated space
comes from Go os.newFile. Reused transfer is 99.63% native syscall CPU with zero
measured per-operation allocation; its memory profile records startup/profile
encoding allocations. Exact commands, iterations and artifact digests are bound
in pipe-benchmark-records.json.


## Held identity and deterministic stream handoff checkpoint

Twenty-two native identity rows, eight Unix permission rows, and a 25-seed
semantic fuzz target pressure held file/directory/pipe custody. Hard links,
identical bytes on foreign inodes, final symlinks, ancestor links, missing and
non-directory ancestors, native permission failures, nil/closed handles and
cancellation retain exact observations and errors. Observation cannot consume
the held cursor or change namespace bytes, identity, modes or timestamps.
Production remains Go Stat/Lstat/SameFile; documentation now describes only
mechanics and the limits of a momentary observation.

All twelve held-standing mutations and two stream-context mutations were
killed without build failures. Seven direct bounded-copy rows independently
pin cancellation before reading, during reading, after destination writes and
before overflow probing, including exact byte receipts and unconsumed input.
This closes the scheduler-dependent coverage gap recorded in the pipe phase;
it is a test ratchet over unchanged production, not a production timing fix.

The package checkpoint passed 1,884 Go events with zero failures/skips and
88.8% statement coverage. Linux and Windows amd64 test binaries compiled.
The thirty-second held-standing fuzz campaign completed 13,799 executions.

The initial three-observation benchmark (Same/Replaced/Absent) measured
8,495 ns/op, 1,632 B/op and 10 allocations over 4,284,738 iterations, with
30-second CPU1 CPU/memory profiles and the retained binary. Native syscalls
account for 99.18% of sampled CPU; sampled allocation is in Go Stat/Lstat and
native path conversion. This is an initial workload, not a speedup claim.
Exact commands and artifact digests are in held-standing-benchmark-records.json;
stream-context-handoff-deterministic-proof.json binds the context ratchet.


## Sharing and public enum closeout

Seven admission rows separate nil/panicking/nil-safe/canceled/expired context,
zero path and actual platform observation. The native Windows table adds same-
process contention, released custody, absent leaf/parent and directory refusal.
A native exclusive probe after each operation detects leaked probe handles.
Unchanged inode, mode, timestamp, bytes, namespace and held read cursor are
checked directly. Windows tests compiled but were not executed on this Darwin
host; their runtime evidence remains unavailable.

The semantic fuzz target begins with a real public OpenRead seed and pressures
bounded payloads, held readers, missing names and all ingress states. The first
seed fixture failed to compile because it passed Location instead of the owning
ReadHandleRequest; that failure and correction are retained separately. The
corrected control passed 20 Go events, and 30 seconds of fuzzing completed
13,433 executions. Seven deliberate sharing/enum mutations were killed without
build failure. Linux and Windows test binaries compiled.

Sharing documentation now correctly describes any conflicting handle, including
same-process handles, without product lock policy. The Darwin profile showed
all per-operation allocation came from building the unsupported-platform error.
That branch now directly returns the existing core-owned Contract identity.
There is no fabricated sharing implementation or global cached error state.

The same 30-second CPU1 workload measured 123.5 ns/op, 72 B/op, 3 allocations before and 42.48 ns/op, 0 B/op, 0 allocations after. This is unsupported-platform refusal, not Windows I/O throughput; the single pair is not a timing trend. CPU/memory artifacts, commands and sample counts are bound in sharing-benchmark-comparison.json.

Seven concrete public enum tests replace the generic assertion helper and the
weaker duplicate InstallMode/HeldStanding/Sharing/PathKind tests. Each typed
domain exhausts uint8 membership, exact refusal and diagnostics, unique admitted
labels, and absence of JSON encoding/decoding. Their 1,799 Go events are not
1,799 distinct semantic quota cases. Internal discriminators and the remaining
legacy sweep are tracked separately.


## Direct tables, maintenance reduction, and Walk cancellation

The closeout replaces the remaining operation closures in context and stream
extent tests with direct table executors. Public Read checks exact partial
acknowledgments, invalid writer counts, overflow prefixes, ownership refusals
and unchanged native files. Resolved Write checks exact zero recovery structs,
source consumption, retained file identity/metadata/bytes, staging conflicts,
create/replace behavior, empty files and native failure causes. The old duplicate
examples are retired explicitly in closeout-maintenance-reductions.json.

Custody validation no longer uses a hierarchy of closure wrappers. Timestamp
rows reach math.MinInt64/math.MaxInt64 nanoseconds and their immediate neighbors,
epoch crossings, and the 32-bit second boundary. Actual Go Chtimes/Stat supplies
the platform precision oracle, while the table checks unchanged bytes/inodes and
Inspect's exact observed timestamp. Internal enum domains are separate concrete
tables; the old handwritten public-API string inventory is removed in favor of
the compiler-bound ingress and data-flow inventories.

Crossing the same Walk refusal table with both orders found a real bug: native
order delivered the rest of a batch after cancellation inside Visit. A second
row and semantic seed also showed both orders could attempt directory acquisition
after cancellation, returning a source failure instead of cancellation. The
retained red run has three failing table rows and one failing fuzz seed (parent
failures are not separate regressions). Two direct context checks now guard the
next native entry and each directory acquisition. Go still owns acquisition and
ReadDir; no scheduler, state machine, or new execution abstraction was added.
The corrected source-bound control passed 191 Go events with zero failures/skips.

The user narrowed platform scope to macOS, Linux and Windows. Other-platform
fallback implementations are removed, Unix implementation tags are narrowed,
and Windows leaves are named explicitly. Fresh platform compilation is part of
the remaining closeout; historical evidence keeps its original paths and bytes.

Test build/oracle mistakes are preserved in closeout-test-corrections.json,
separately from real production reds. The Write missing-parent fixture originally
violated the same-parent request contract; the Walk regular-replacement oracle
originally guessed fs.ErrInvalid instead of obtaining Go's native acquisition
cause. Neither mistake is counted as a production regression.
