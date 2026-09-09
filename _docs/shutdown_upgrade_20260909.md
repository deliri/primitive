# Shutdown upgrade — approved for v2026.1.32

The user reviewed this slice and approved bump, commit, push, and moving to Currency.
The execution-time notes below remain unchanged historical evidence. See
[release notes](release_v2026.1.32.md) for the approved checkpoint.

# Shutdown review follow-up — ready for review — 2026-09-09

The two reported bugs and both additional review items are addressed.
This section supersedes the original candidate notes preserved below.
Base remains `v2026.1.31`, `567adddc7d5e552258f7034ef395cd41ca414f9d`.
The final dirty Go-source inventory digest is `264375762f64d4cb44845b6cccb94e7f1dcaf2336b6444c4e74bf5c602e029ac`.
All final scoped checks, platform compilation, benchmarks and fuzz campaigns
match those exact package bytes. Go 1.27.1, Furnace, GOWORK=off, GOMAXPROCS=8.

## Review corrections

1. **Observer reentry:** construction captures the parent's Done channel inside
   the recovered boundary. The observer selects that channel and never calls
   the parent's Done again after first-signal unlinking. The red test crashed
   the test process; the fixed tests arm a hostile parent after unlinking and
   prove exact second-signal escalation without any additional Done calls.
2. **Panic identity and cleanup:** new `ContextPanicError` preserves the original
   error through Unwrap, gives bounded UTF-8 diagnostics and never invokes the
   recovered error's formatter. It matches ErrContextObservation only when
   the original panic error does. A construction failure cancels an acquired
   child handle before refusing the controller.
3. **Terminal facts:** context observation failures, cancellation, deadline and
   custom cancellation causes remain distinct. Skipped callbacks remain unrun;
   started callback failures remain reachable alongside the observed terminal
   fact. Public Run still creates its own standard deadline root; direct helper
   tests explicitly attack the wider canceled/invalid-context input domain.
4. **Rejected start:** beginRun validates under Plan's existing mutex before
   claiming single use. Rejected policy/count validation does not consume it.
   The same mutex now owns registration and the plain started bool; the atomic
   flag/import was removed. Two and 64 competing starts each execute once.

Reading Go's context source also exposed parent Done/Value calls in removeChild.
Close and the owned signal goroutine now contain those unlink panics, finish
the join and release once, and retain the original failure for repeated Close.
The tests remove each guard in turn and reproduce the panic. The child is
already canceled when Go reaches that unlink path, so its first cause remains
authoritative even when cleanup fails.

Production changes now span plan.go, signal.go and errors.go. The compiler-linked
struct inventory has 17 types, including the new typed error. There is still
one owned observer goroutine, fixed signal buffers, a 64-step maximum, and
cooperative synchronous callbacks. Go context and os/signal continue to own
propagation and subscription; Temporal owns timer construction. This does not
promise to contain invalid caller Context methods executing inside Go's own
fallback propagation goroutines. No custom propagation runtime was introduced.

## Hostile proof and validation

The new review_boundary_test.go adds earned tables for late Done, native and
wrapped panic identity, hostile Error formatting, skipped canceled/deadline
roots, invalid starts, concurrent starts, unlinking, the error schema triad and
custom cause observation. FuzzWatchParentPanicIngress mutates strings and bytes
through the constructor, checks an independent first-256-runes diagnostic
oracle, and retains an unmutated source-closure control. The previous callback
and signal producer/classifier triads remain active.

- Linux final race run: **259 passing test events**
  (including parents), **49 top-level functions**, zero failures/skips,
  **96.4% statement coverage**.
- Scoped go vet, staticcheck, errcheck, witness-lint and production gocyclo <=10
  pass. macOS arm64 and Windows amd64 test binaries compile; neither was run.
- Four follow-up behavioral mutations were killed: unguarded observer unlink,
  unguarded Close unlink, missing parent-channel exit, and lost custom cause.
  The earlier five mutation checks remain historical proof.
- Two channel-mutation attempts are explicitly excluded: one did not compile;
  one used a nonexistent test filter and ran no tests. The corrected mutation
  compiled and failed because parent cancellation incorrectly emitted a grace
  escalation. All mutations were restored before final checks.
- Review-red-identities and review-red-late-done retain the original red results.
  Review-witness retains the initial lint failures; subsequent corrections and
  final pass are recorded. No failed or empty run is relabeled as a pass.
- Full-module gates and other packages were not run.

The one select waiver names the captured parent Done channel and Close stop
channel directly. Calling parent.Done solely to satisfy a syntactic lint would
reintroduce the reviewed bug. The two context.Value fixture waivers implement
the standard library's required interface signature.

## Fresh benchmark evidence

The user waived further Shutdown benchmarking after this pass had already
completed. No additional benchmark pass is required or planned. Existing completed
evidence is preserved below.

The benchmark harness is byte-identical across original, reviewed candidate
and this follow-up. Each revision has one pass, six cases at 30s each, with
CPU/memory profiles and the matching executable retained. Exact argv, source
snapshots and all outputs are in the machine report.

| Workload | Original ns/op | Reviewed ns/op | Follow-up ns/op | Change vs reviewed | B/op original / reviewed / follow-up | Allocs/op original / reviewed / follow-up |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `BenchmarkPlanLifecycle/empty` | 2524 | 2566 | 2747 | +7.05% | 2080 / 2080 / 2080 | 6 / 6 / 6 |
| `BenchmarkPlanLifecycle/one` | 4954 | 5349 | 4382 | -18.08% | 2544 / 2544 / 2544 | 11 / 11 / 11 |
| `BenchmarkPlanLifecycle/maximum` | 72987 | 78028 | 56098 | -28.11% | 8592 / 8592 / 8592 | 137 / 137 / 137 |
| `BenchmarkPlanLifecycle/maximum-failures` | 175376 | 187247 | 136759 | -26.96% | 28563 / 28563 / 28563 | 713 / 713 / 713 |
| `BenchmarkReportWalkMaximum` | 2528 | 2525 | 2757 | +9.19% | 0 / 0 / 0 | 0 / 0 / 0 |
| `BenchmarkWatchClose` | 112125 | 120042 | 66291 | -44.78% | 874 / 877 / 861 | 11 / 11 / 11 |

Fresh phase: 180s nominal, **214.491s process wall**;
wall/per-case flag **7.15x**, wall/phase budget **1.19x**.
Every case exceeded 30 timed seconds. This is one observation per revision,
not a statistical speed claim. Other Anvil test jobs were observed on Furnace
during this pass and that host-load sample is retained; no CPU affinity or power
isolation was applied. Timing differences cannot be attributed to the patch
from these measurements alone. Allocation differences remain visible.

The aggregate profile's relevant rows are retained verbatim here:

```text
cpu: 29.87s 11.48% 11.48%     29.87s 11.48%  runtime.futex
cpu: 11.33s  4.35% 26.49%    117.75s 45.25%  github.com/deliri/primitive/v2026/shutdown.(*Plan).Run
cpu: 1.86s  0.71% 53.43%     11.61s  4.46%  github.com/deliri/primitive/v2026/shutdown.NewPlan (inline)
cpu: 0.72s  0.28% 69.05%     48.22s 18.53%  github.com/deliri/primitive/v2026/temporal.WithTimeout
cpu: 0.55s  0.21% 72.29%     19.89s  7.64%  runtime.futexsleep
cpu: 0.16s 0.061% 77.07%     10.69s  4.11%  runtime.futexwakeup
cpu-cumulative: 11.33s  4.35%  6.29%    117.75s 45.25%  github.com/deliri/primitive/v2026/shutdown.(*Plan).Run
cpu-cumulative: 0.72s  0.28%  9.57%     48.22s 18.53%  github.com/deliri/primitive/v2026/temporal.WithTimeout
cpu-cumulative: 29.87s 11.48% 33.03%     29.87s 11.48%  runtime.futex
cpu-cumulative: 0.55s  0.21% 33.97%     19.89s  7.64%  runtime.futexsleep
cpu-cumulative: 1.86s  0.71% 37.49%     11.61s  4.46%  github.com/deliri/primitive/v2026/shutdown.NewPlan (inline)
cpu-cumulative: 0.16s 0.061% 37.72%     10.69s  4.11%  runtime.futexwakeup
mem: 36.23GB 64.10% 64.10%    36.23GB 64.10%  github.com/deliri/primitive/v2026/shutdown.NewPlan (inline)
mem: 0     0% 99.54%    19.88GB 35.17%  github.com/deliri/primitive/v2026/shutdown.(*Plan).Run
mem: 0     0% 99.54%    14.86GB 26.29%  github.com/deliri/primitive/v2026/temporal.WithTimeout
```

These profiles describe standard runtime/context work and explicit fixed-capacity
allocation. They do not justify adding a pool or competing scheduler.
alloc_space measures cumulative sampled allocations, not peak RSS.

## Fresh semantic fuzz campaigns

All six targets ran serially after benchmarks, with -run=^$,
-fuzz=^<target>$, -fuzztime=30s, -parallel=4, -fuzzminimizetime=1s and
-timeout=2m. Persistent caches were not cleared; full baseline-gathering and
progress output is retained. No target was skipped, retried or failed.

| Target | Executions | Process wall | Result |
| --- | ---: | ---: | --- |
| `FuzzStepIDNominalIngress` | 1,761,041 | 31.995s | passed |
| `FuzzPlanRegistrationAndExecution` | 755,409 | 30.572s | passed |
| `FuzzCallbackPanicDiagnosticIngress` | 1,360,785 | 30.515s | passed |
| `FuzzWatchPolicyAndSourceClosure` | 1,538,330 | 30.484s | passed |
| `FuzzNativeSignalObservation` | 892,714 | 30.485s | passed |
| `FuzzWatchParentPanicIngress` | 1,366,171 | 30.491s | passed |

Total **7,674,450 executions**;
180s nominal, **184.543s process wall**; wall/per-target flag
**6.15x**, wall/phase budget **1.03x**.
No failing corpus was produced.

## Current review surface

- Current manifest: [review follow-up evidence](shutdown_upgrade_20260909_review_followup.json).
- Findings and historical candidate are retained under
  `/home/d/engineering-evidence/primitive/shutdown-upgrade-20260909`; large binaries and profiles stay there.
- The manifest binds each run to the base commit plus complete source inventory,
  retained stdout/stderr, artifacts and tool arguments; 4,265
  distinct source snapshots and all run artifacts were rehashed.
- No bump, commit, push or consumer update has occurred. This is author-produced
  evidence awaiting user review, not an independent acceptance receipt.
  Currency follows the reviewed Shutdown checkpoint.

---

## Historical candidate — superseded by the follow-up above

The original notes below describe the pre-review implementation. In particular,
their unconditional ErrContextObservation claim, 16-struct count, two-production-file
scope and earlier verification numbers are historical, not current claims.

# Shutdown upgrade — ready for review — 2026-09-09

Base: `v2026.1.31`, `567adddc7d5e552258f7034ef395cd41ca414f9d`. Work was performed on
Furnace (`d@192.168.1.81`, `/home/d/code/primitive`) with Go 1.27.1.
Candidate Go-source inventory digest: `2af3d3f67dc92a68610a0c28a6ab4f5d26980b2729e8518cd1eb8b820e32acd9`.
The complete 2,214-line project testing protocol was read before test edits.
This is author-produced execution evidence awaiting review, not an independent
acceptance receipt. No version bump, commit, push, or consumer update occurred.

## Production changes and demonstrated red states

Three production defects were reproduced before their fixes:

1. An unconstructed `Plan` admitted a valid cleanup step even though its policy
   could never run. `Register` now asks the owning `PlanPolicy.Validate` before
   retaining work. The table proves rejected admission retains no callback or
   count, while the same step in a constructed plan executes exactly once.
2. A skipped step after total expiry carried `ErrShutdownTotalTimeout` but lost
   `context.DeadlineExceeded`. Every skipped observation now retains both
   identities while proving its callback never ran. The started-step cause is
   preserved separately.
3. `Watch` validated a parent using `Err`, then could panic when Go's context
   propagation called its `Done` or `Value`. Construction now contains that
   panic as `ErrShutdownContract` plus `ErrContextObservation`, returning no
   controller so `Watch` releases its acquired `os/signal` subscription.
   Both the injected constructor and real public subscription path are tested.

The signal-buffer comment also stopped claiming lossless delivery. A buffer of
size two follows Go's actual `os/signal` contract: delivery may coalesce or drop
under a burst. No replacement delivery system was added.

Production changed only in `shutdown/plan.go` and `shutdown/signal.go`.
There are no new production imports, types, APIs, background workers, queues,
or shared contracts. Existing Core error identities remain the authority.
Plan registration is mutex-owned; execution remains single-use, synchronous,
phase ordered and LIFO within a phase. Its capacity remains `MaximumSteps` (64).
Controller owns one Go signal subscription and one joined goroutine. Timeouts
still go through Temporal to Go contexts. Callback budgets remain cooperative:
a callback must return before Run can finish. This is not forced termination.

## Public boundaries and proof surfaces

| Surface | Proof |
| --- | --- |
| `NewStepID`, Validate, String | Numeric boundary table; exact nominal-value fuzz oracle and adjacent-identity distinction |
| Seven off-wire enums | All 256 underlying byte values for each enum; exact admission and distinct known labels; no byte decoder is advertised |
| Step / PlanPolicy / NewPlan / Register | Named field and budget bounds, zero/nil, duplicate, capacity and closed-registration refusals; three registration-specific duplicate rows removed |
| Concurrent Register → Run | Below/at/above capacity and double-capacity contention; admitted identities equal executed identities equal retained identities, exactly once |
| Callback producer → outcome classifier | Nil/error/panic exits × active/step-expired/total-expired states; before/at/after real Go timer boundaries in `testing/synctest`; exact callback facts and exclusive outcome identities |
| Plan.Run / single use / reentry | Exact phase/LIFO output, native error aggregation, terminal-parent detachment/value preservation, rejected nil parent with no execution, explicit reentry error capture |
| Empty and skipped accounting | `TestShutdownCallbackProducerClassifierLayerTriad` seals an empty report without extra observations; skipped-step table proves absence of execution and preserved timeout identity |
| StepResult / Report schema / Result | Outcome/error mismatch tests, unset/corrupt count/result refusal, bounded lookup, empty sealed report; report-local `LayerTriad` |
| StepAction panic boundary | Real callbacks panic with error, hostile formatter, string, bytes, numeric and other values; exact bounded UTF-8 diagnostic fuzz oracle |
| SignalPolicy / WatchRequest | Closed action/grace cross-product with an independent expected domain; parent/policy/set refusal; constructor/source-closure semantic fuzz |
| Watch / Controller / Close | Real native SIGINT/SIGTERM/SIGHUP, first and second signal identities; typed process-wide serial declarations, guarded native injection, joined release |
| Signal producer / cause / escalation | Unknown input remains neutral; first signal authenticates exact kind; source loss remains typed refusal; close/parent/source/second/grace exits retain exact facts and release exactly once |
| Platform observation ingress | Native-value fuzz through the owned injected source and real observer goroutine, separate Unix and Windows expectations; native integration tests exercise the real OS subscription |
| Structural ownership | Embedded source instead of direct filesystem reads; compiler-linked 16-struct role inventory, one-goroutine/import frontier, AST effect/collection matcher and constructor/fuzz inventory |

The callback/classifier transition domain has three callback exits and three
observable budget states. The nine semantic handoffs are exhausted; the time
boundary probes supplement them. We do not claim an invented 50-case matrix.
Exclusive primary labels distinguish refusal, competing panic/expiry candidates,
boundary observations and the neutral empty run. Native signal fuzz uses the
owned source seam and does not claim to generate millions of real OS signals;
the serial native tables are the actual OS integration proof.

The package owns no durable writer, wire/JSON decoder, ledger, manifest
projection, or reporter. Those protocol layers are not fabricated here.
The ingress inventory names constructors and their fuzz owners, refuses new
Parse/Decode/Read/Load/Replay/Unmarshal doors without review, and is mutation-tested.
Existing small platform projections and enum domains remain exhaustively tested.
The callback result retains normal Go error identities and references; it does
not clone arbitrary caller-owned error objects.

## Removed weak proof

The former reentry callback returned the unexpected error as its own result.
An incorrect nil rejection therefore returned nil and could pass. The new
reentry table observes the rejection independently and the deliberately removed
registration guard now kills the test. The old partial accounting test was
replaced by local outcome/accounting triads, including an empty run and exact
callback counts. An impossible unsigned `index < 0` branch disappeared with it.
The old benchmark included `testing.TB` fixture helpers in its timed loop; the
new harness prepares typed step fixtures before timing and checks exact work.
The subprocess signal test's loose readiness handshake and unjoined early-error
paths were replaced with six serial native-signal cases with owned cleanup.

## Validation and attempt accounting

- Linux race execution: **209 passing test events (including parents), 40
  top-level test/fuzz functions, zero failed or skipped events** in the closing
  run; **96.5% statement coverage**. This is not a claim of 100% coverage.
- Scoped `go vet`, `staticcheck`, `errcheck`, and `witness-lint` passed.
  Production `gocyclo -over 10` produced no offenders. Formatting and
  `git diff --check` are clean.
- macOS arm64 and Windows amd64 test binaries compiled. They were not executed.
  The Windows native-signal fuzzer was added after the closing Linux race run;
  it compiled separately and the final benchmark/fuzz binaries use the final
  package source. The exact one-file race-snapshot delta is in the manifest;
  Linux production and executable test bodies did not change.
- Five intentional mutations failed for their advertised behavior, not build
  errors: reverse LIFO order, remove panic precedence, admit callback reentry,
  add an unclassified production struct, and add an unfuzzed decoder. All were
  restored before closing checks and final measurements.
- Full-module gates and the other packages were not run in this slice.

Every attempt remains in the machine report and raw directory. Apart from the
three production red states, one green attempt failed to compile after test
retirement left an unused import; that import was removed. An initial Witness
invocation used an unavailable source checkout; the installed binary then found
a mandatory `context.Context.Value` interface signature and an insufficient
failure diagnostic. The signature now has the existing canonical waiver and
the diagnostic includes the observed value; Witness passes.

One race attempt also exposed a fixture defect: equal independent parent/child
deadlines allowed child Done before the root timer callback. Observing the
child did not prove total expiry. The skipped-step fixture now gives the step
a longer budget so Go clips it to the parent's cancellation. The failure is
retained, not counted as a fourth production defect or hidden as a retry.

## Comparable before/after benchmarks

The upgraded benchmark harness was installed and the original production
measured **before any production edit**. Its exact file hash matches the
candidate harness. Both phases used the same machine, Go 1.27.1,
`GOWORK=off`, `GOMAXPROCS=8`, and this argument shape:

```text
go test -run=^$ -bench=. -benchmem -benchtime=30s -count=1 -timeout=10m \
  -cpuprofile=<phase>/cpu.pprof -memprofile=<phase>/mem.pprof \
  -o <phase>/shutdown.test ./shutdown
```

| Workload | Before ns/op | Candidate ns/op | Observed change | B/op before → candidate | Allocs/op before → candidate |
| --- | ---: | ---: | ---: | ---: | ---: |
| `BenchmarkPlanLifecycle/empty` | 2524 | 2566 | +1.66% | 2080 → 2080 | 6 → 6 |
| `BenchmarkPlanLifecycle/one` | 4954 | 5349 | +7.97% | 2544 → 2544 | 11 → 11 |
| `BenchmarkPlanLifecycle/maximum` | 72987 | 78028 | +6.91% | 8592 → 8592 | 137 → 137 |
| `BenchmarkPlanLifecycle/maximum-failures` | 175376 | 187247 | +6.77% | 28563 → 28563 | 713 → 713 |
| `BenchmarkReportWalkMaximum` | 2528 | 2525 | -0.12% | 0 → 0 | 0 → 0 |
| `BenchmarkWatchClose` | 112125 | 120042 | +7.06% | 874 → 877 | 11 → 11 |

Each row has one independent sample per revision, with iterations and effective
timed duration retained in JSON. All six rows exceeded 30 timed seconds; none
hit Go's iteration ceiling. This is **not a statistical speed claim**. The
nonempty lifecycle and Watch samples are slower by about 7–8%; the report walk
is essentially unchanged. No timing regression is dismissed as proven noise,
and no speedup is claimed. Allocation counts did not increase; Watch's small
amortized byte difference is visible above.

The host was shared, without CPU affinity or power isolation. `schedutil` was
sampled at setup and after all measurements, not continuously or at both start
instants. Original and candidate binaries, CPU profiles, memory profiles and
full output are retained and hashed. The historical reservation-audit benchmark
is not a comparable baseline for this machine and workload.

Six × 30s means a nominal **180s per benchmark phase**; actual process wall time
was **216.477s before / 220.231s candidate**.
The wall/per-case-flag ratios are **7.22× / 7.34×**;
wall/declared-phase ratios are **1.20× / 1.22×**.
There was one benchmark pass per phase, no favorable-sample selection.

## What the profiles support

The candidate aggregate CPU profile attributes 13.28% flat to Go runtime futex
work, 17.90% cumulative to Temporal.WithTimeout/Go context construction and
4.60% flat to Plan.Run. The baseline shows the same broad paths: futex 13.15%
flat and Temporal.WithTimeout 18.52% cumulative. These aggregate proportions do
not establish the cause of the per-operation timing changes.

Candidate sampled allocation space is dominated by the explicit fixed-capacity
Plan allocation (69.11%) and standard context/timer lifetimes. These are the
actual owned mechanics. A pool, extra scheduler, duplicate timeout runtime or
mutable reusable plan would add ownership complexity without evidence that it
belongs in this once-per-lifecycle capability, so none was introduced.
The profile's cumulative allocation volume is **not** peak RSS or retained heap.
The 64-result walk allocates zero bytes in both observations.

## Semantic fuzz evidence

Every target ran once, serially after tools/tests/benchmarks, with:

```text
go test -json -run=^$ -fuzz=^<target>$ -fuzztime=30s \
  -fuzzminimizetime=1s -parallel=4 -timeout=2m ./shutdown
```

| Target | Executions | Process wall time | Result |
| --- | ---: | ---: | --- |
| `FuzzCallbackPanicDiagnosticIngress` | 1,326,038 | 30.556s | passed |
| `FuzzNativeSignalObservation` | 748,141 | 30.439s | passed |
| `FuzzPlanRegistrationAndExecution` | 460,655 | 30.583s | passed |
| `FuzzStepIDNominalIngress` | 1,817,229 | 31.584s | passed |
| `FuzzWatchPolicyAndSourceClosure` | 1,513,954 | 30.402s | passed |

Total: **5,866,017 executions**. Configured fuzz
budget: **150s** across five targets; effective process wall:
**153.564s**. Persistent fuzz caches were not
cleared; complete baseline-gathering and mutation progress is retained.
No target failed or produced a crasher. New interesting-cache entries are not
promoted as accepted evidence or committed corpus without a failure to preserve.

## Review artifacts and checkpoint

- Machine report: `_docs/shutdown_upgrade_20260909_evidence.json`.
- Raw evidence: `/home/d/engineering-evidence/primitive/shutdown-upgrade-20260909`.
- Both profile pairs and corresponding test binaries remain on Furnace.
- The machine report binds commands, source snapshots, failures, passes,
  tool identities, stdout/stderr and artifact hashes; all referenced raw files
  and source snapshots were independently rehashed by the report builder.
- The report builder is reproducible in the raw directory. It is an artifact
  inventory tool, not an independent verifier authority or acceptance issuer.

Review this Shutdown slice before a release bump or commit. Currency is next in
the saved queue, after this package's reviewed checkpoint.
