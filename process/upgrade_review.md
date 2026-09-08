# Process upgrade review

Process package work is complete for the available Darwin host. Core and
Contextstate, the batch gates, and explicit user review remain before release.
The base is `v2026.1.20` (`1e1adb36cc784d10038d384bac6c775b3b51326d`).
The complete 2,214-line `_docs/testing_protocol.md` was read before test edits.

## Production changes and proof

- Exact environment admission now checks that every supplied pair survives
  Go's `exec.Cmd.Environ` projection in order. Length comparison alone missed
  collisions when Windows critical-variable insertion offset deduplication.
  Forty hostile admission rows, forty replacement/ownership rows, independent
  semantic fuzzing, a compiled collision-acceptance mutation, and an installed
  Go critical-variable branch replay exercise this boundary. Go owns name
  identity and deduplication; Primitive adds no environment map.
- Unix process IDs are checked against the signed native PID domain before
  syscall narrowing. Three high unsigned IDs previously crossed that boundary.
  Tests exercise conversion and signal-zero observation; they never send a real
  signal to a hostile PID. Group sweep preserves EPERM rather than claiming a
  successful effect. Eight finite errno outcomes pin this distinction.
- Windows liveness waits on a synchronized native handle instead of treating
  exit code 259 as proof of life. Windows result tests retain a second handle
  to prevent completed-object deletion and PID reuse. Both public kill doors
  own reaping cleanup and refuse post-reap delivery. These tests compile here;
  native Windows execution is still required for runtime evidence.
- Results retain the complete unsigned Windows exit domain through an `int64`
  observation. Optional peak memory distinguishes unreported RSS from measured
  zero. Unsupported Windows RSS no longer discards the child's other facts.
  Validation rejects contradictory signals, exits, counters, and availability.
  Observation pointers are independently owned. Runnercontrol and Machineprobe
  consume the changed contract; the obsolete numeric conversion was removed.
- Plans admit only vectors that fit the shared strict JSON array limit and
  prove canonical publication size before assigning a decoded receiver. Tests
  cover exact limits, omitted fields, expansion, malformed input, binding and
  alias custody, and unchanged receivers on refusal. Go's encoder owns escaping.
- Native execution remains `os/exec`; syscall leaves now use Go's standard
  library. An impossible invalid-native-ID cleanup path retains Wait's error.
  No process registry, scheduling engine, product state, or replacement runtime
  was added. Group sweep documentation states the OS group-name reuse limit.

## Test and fuzz improvements

Direct table verdicts replace hidden assertion helpers and DeepEqual checks.
Tests distinguish refusal from zero or partial publication and verify exact
bytes, ownership, native error identity, reader/writer counts, independent stream
limits, no-progress handling, and nil/zero execution handles. Vector and argv
fixtures use Go gob framing so embedded separator bytes cannot hide corruption.
Environment limit fixtures use unique names so collision refusal cannot mask a
broken bound. Test timers use Temporal, and child fixture errors are handled.

Eleven semantic fuzz targets bind twenty public operation witnesses plus two
native/private doors. Every final campaign ran serially with one worker, a
30-second fuzz budget, and `-fuzzminimizetime=1s`. Cached interesting inputs were
snapshotted before and after these final runs. All passed:

| Boundary | Final executions |
| --- | ---: |
| Argument/environment atoms | 309,990 |
| Arguments and ambient environment | 139,265 |
| Exact environment | 216,079 |
| Effective environment | 244,203 |
| Plan JSON | 164,897 |
| Result observation | 440,036 |
| Truncating writer | 597,200 |
| Run and Begin streaming | 2,269 |
| Resolution | 13,362 |
| Native process sighting | 439,325 |
| Stream output | 450,195 |
| Go critical-variable replay: exact environment | 286,007 |
| Go critical-variable replay: effective environment | 268,110 |

Initial campaigns also passed and remain retained. They used Go's default
minimization allowance and did not snapshot the existing interesting-input
cache. Progress plateaus prompted a diagnostic campaign using Go's own debug
and minimization flags. Its logs show coverage minimization; a native worker
sample with unresolved Go symbols does not establish a production CPU defect.
The replay is not native Windows execution, and counters are not a statistical
fuzz-throughput comparison.

## Benchmarks and profiles

Each final before/after workload uses the same frozen v5 harness, a single
30-second sample, and its own CPU profile, memory profile, and matching binary.
Before overlays restore original production; they do not substitute a weaker
benchmark. All fourteen runs and twenty-eight CPU/alloc_space summaries passed.

| Workload | Before ns/op | After ns/op | Before B/op; allocs/op | After B/op; allocs/op |
| --- | ---: | ---: | ---: | ---: |
| stdout 64 KiB | 7,189,217 | 8,124,169 | 124,550; 79 | 124,547; 80 |
| stdout 1 MiB | 7,949,342 | 8,101,408 | 124,537; 79 | 124,534; 80 |
| stdin 64 KiB | 11,376,949 | 11,471,605 | 124,505; 79 | 124,541; 80 |
| stdin 1 MiB | 13,164,983 | 10,894,976 | 124,535; 79 | 124,530; 80 |
| Result observation | 42.27 | 59.68 | 0; 0 | 8; 1 |
| Plan round trip | 11,334 | 12,302 | 4,333; 75 | 5,148; 90 |
| Exact environment, maximum | 757,195 | 786,017 | 578,886; 4,116 | 578,886; 4,116 |

These are measurements, not a general speedup claim. Streaming allocation is
flat across the tested payload sizes; Go's `io.copyBuffer` accounts for roughly
78–80% of allocated space. Parent CPU profiles contain syscall waiting and Go
scheduler work and do not measure subprocess CPU. Result observation's 8-byte
allocation owns the optional RSS fact. Plan's extra work proves canonical
publication through the Go encoder. Environment allocation comes from Go's
deduplication and bounded vector projection; argv/environment operations are
bounded O(n), not O(1) memory. `alloc_space` is cumulative allocation, not RSS.

An earlier environment projection-reuse experiment reduced allocation but was
slower at all three measured sizes, so it was reverted. Original/retained/rejected
ns/op were 337.3/375.3/430.2 at one variable, 20,191/20,878/23,114 at 64, and
1,355,816/1,414,317/1,626,863 at 4,096. All intermediate runs remain in the
manifest; labels containing `environment-final` identify that rejected attempt.

## Verification and limits

Native ordinary and race runs pass 1,586 test events across Process,
Runnercontrol and Machineprobe, with no failures or skips. Events include parent
and subtests and are not unique-case counts. Process statement coverage rose
from 84.4% to 90.8%. Focused vet, staticcheck, witness-lint, requested errcheck,
Core export ownership checks, and Process complexity checks pass. Linux/amd64
and Windows/amd64 test binaries compile. Neither platform was executed here.
A broader errcheck diagnostic with extra `-blank -asserts` flags remains recorded
with receiver-test baselines and an `errors.AsType` blank-result false positive.

The Blink Kernel fixture was migrated for workspace compatibility and 543 Anvil
test events passed. That is not published dependency adoption; its dependency
version has not been bumped. No new Process changes have been committed.

[upgrade_evidence.json](upgrade_evidence.json) binds commands, source snapshots,
outputs, expected production reds, fixture/build corrections, profiles and
binaries. Raw artifacts remain ignored under `testdata/test-upgrade-20260908/`.
Only reports and the ignore rule belong in the reviewed commit. Failed attempts
are retained and distinguished from production regressions. Focused checks do
not stand in for the later batch gates or user approval.

The subsequent Core integration run initially failed its native-effect inventory because it still expected Process to import x/sys. Core's inventory now records the actual syscall leaves and retains forbidden-owner checks. The corrected Core suite and its race run pass; this was an integration-test inventory fix, not another Process production change. The failed and corrected executions are retained in `core/upgrade_evidence.json` (`core-original-tests`, `core-effect-ownership-corrected`, and `core-race-final`).
