# Temporal and Hostfacts review follow-up

The updated review was found at `/tmp/temporal-hostfacts-bug-report.md`, last
modified at 23:27 on September 7, 2026. The requested underscore filename still
did not exist at the final check. Both report versions are retained locally.
The update added independent-review qualifications, not new findings.

The five actionable findings below are addressed and ready for user review.
No version bump, commit or push has been performed. This follow-up supersedes
the current-source claims in the earlier upgrade reports; their measurements
remain historical evidence.

## Findings and resulting behavior

| Finding | Result and evidence |
| --- | --- |
| H2: missing cgroup directory becomes an absent declaration | Directory acquisition must succeed through Filestore before a missing interface can mean absence. Ten failing filesystem cases reproduced incorrect v1/v2 ancestor limits or unavailable results. The replacement refuses vanished leaf, intermediate and mount directories with a zero result and the native cause. Existing directories without interface files remain valid. |
| H1: cgroup failures lose their cause | Core now owns six closed error identities for disappeared, changed or duplicate membership, missing or ambiguous mounts, and containment failure. Actual bounded proc-file fixtures distinguish read failure, disappearance, controller/path movement and duplicate declarations. Mount fixtures distinguish absence, ambiguity, specificity and malformed trailing data. Nine diagnosis rows failed before the change. |
| H3: unsupported physical memory becomes an observation failure | The native-result admission preserves `ErrHostFactsUnsupported` as `Failure.Identity`, including wrapped causes. Ordinary native failures remain observation failures and discard partial totals. Windows still has no physical-memory adapter; this change does not claim to implement one. |
| H4: caller-invalid ambient input becomes an observation failure | Invalid environment names and path text now retain Hostfacts and owning-package contract identities, without Hostfacts observation identity. Oversized values read from the OS remain observation failures. The lookup table, explicit path table and path fuzz oracle exercise that distinction. |
| T1: unset ticker access returns a nil channel | Both nil and allocated-zero tickers now panic with `core.ErrTemporalContract`. Valid access returns Go's exact channel; stopping retains it, as `time.Ticker` does. `Validate` returns the typed identity directly, allowing Go to inline the check. No new helper, ticker state or compatibility API remains. |
| H5: ambient/lookup duplicate-key disagreement | Disproved for the installed Go implementation. `syscall.copyenv` removes later duplicates before both `Getenv` and `Environ`. A native `os.StartProcess` probe supplied duplicate entries without `exec.Cmd` deduplication: different values, empty first value and embedded equals all produced the same first-entry result through both public doors. Source and probe are retained. No lookup implementation change was made. |

The shared cgroup read/recheck logic remains ordinary bounded file processing.
Linux supplies its real proc paths. Tests supply real temporary files to the
same functions. There is no injected execution provider or replacement runtime.
The generic virtual-value reader is now located with its sole remaining
production consumer, Linux disk-rotation observation. The changed value-reader paths close acquired files and roots once through
deferred cleanup, preserving close errors.

Membership rereading detects observed membership changes; it does not make the
kernel's multiple cgroup files an atomic snapshot. Directory acquisition cannot
promise that the namespace remains unchanged afterward. The package adds no
retry coordinator or custom filesystem model to disguise that limitation.

The lower-confidence items did not establish additional defects: zero disk
capacity remains outside the existing positive capacity contract; `Fd` acts on
a Hostfacts-owned directory handle; Windows noncanonical environment paths
remain strict-admission refusals without a native Windows reproduction; negative
zero UTC offset and RFC3339 second precision retain their explicit contracts.

## Verification

The complete 2,214-line local testing protocol was read before editing tests.
New cases are direct tables; native errors and shared classifications use
`errors.Is`/`errors.As`. Results, identities, absence and zero-on-refusal are
checked in the case body. No assertion helper or production test hook was added.

- The uncached full-module test gate passed all 61 packages. Hostfacts had 1,018
  passing events and Temporal 744, without failures or skips in either package.
  Three credentialed GCS tests skipped elsewhere. The seven raw Testserial
  failure events are its intentional nested refusal probes; its enclosing
  tests and package passed.
- After relocating the Linux-only reader and tightening cleanup, two shuffled
  race runs passed Hostfacts, Temporal and Core: 2,036, 1,488 and 2,732 passing
  events respectively. Hostfacts statement coverage was 90.2%; Temporal was
  95.2% at that checkpoint. The final Temporal-only inlining change received
  another two passing shuffled race runs. Counts include parents and fuzz seeds.
- Full-module Go fix, vet, Staticcheck, Deadcode, Witness lint, Errcheck,
  NilAway, Goconst, complexity, build and module-tidiness checks passed after
  the resource cleanup. Final Temporal-only changes received focused fix, vet,
  Staticcheck, Witness, Errcheck, NilAway, complexity and race checks. Production
  complexity remains at most ten. Goconst used minimum length four, three uses
  and no tests; its four existing admissions are unchanged. No lint waiver was
  added. This is the requested gate set, not every auxiliary campaign in
  `scripts/gate.sh`.
- Final Hostfacts and Temporal test binaries compile for Linux/amd64 and
  Windows/amd64. Execution evidence is Darwin/arm64 only. Physical-memory error
  classification tests exercise native-result admission; they are not a claim
  of native Windows execution.
- Three affected fuzz targets completed 30-second requests, serially with one
  worker: cgroup limit files 44,490 executions, membership files 17,693, path
  resolution 6,566. Total: 68,749, no failures. The path run's execution counter
  plateaued after three seconds; the complete log is retained, without claiming
  sustained throughput or interpreting that plateau as successful executions.
  Earlier package-wide fuzz campaigns remain separate checkpoint evidence.

All unsuccessful attempts remain in the evidence. They include a test-call-site
build repair, the relocated reader's Staticcheck finding, the benchmark's
explicit nil guard, and an intermediate panic form rejected by Witness.
The first physical-memory table also had an incorrect `MaxUint64` success
expectation: Core's ByteLength uses Go's signed extent. That row was corrected
to maximum-signed success and overflow refusals; it is not counted as a
production defect. The supported/unsupported identity failures were independent
semantic failures in that same run.

## Profiled benchmarks

Each row was measured in its own `go test` process with `-run=^$`, an exact
benchmark selector, `-benchtime=30s`, `-count=1`, `-benchmem`, and both CPU and
memory profile flags. The matching binary was retained with `-o`. Runs were
serial and separate from active tests/analyzers. Go 1.27.1, Darwin/arm64, Apple
M1 Max, default test CPU setting of ten. No cold filesystem-cache claim is made.
These are single samples, not confidence intervals or statistically established
speedups. Exact commands, durations, source snapshots, machine and power
observations are in the [evidence manifest](review_followup_evidence.json).

| Workload | Before | Final | Bytes/op before → final | Allocs/op before → final |
| --- | ---: | ---: | ---: | ---: |
| Three existing cgroup directories, finite middle declaration, absent leaf/root interfaces, exact result check | 291,597 ns/op | 139,534 ns/op | 2,224 → 2,144 | 48 → 44 |
| Ticker channel access and identity check, 64 calls per batch | 0.9481 ns/call | 1.115 ns/call | 0 → 0 | 0 → 0 |

Cgroup CPU samples remain dominated by native syscalls through Filestore and
Go: the final sample attributes 99.38% to `syscall.rawsyscalln`. That supports
leaving execution with those owners. The 52% lower elapsed sample is highly
sensitive to filesystem conditions and is not presented as a dependable
algorithmic speedup. Allocation accounting fell by four objects and 80 bytes
per operation; only failed closes require an extra error join now.

The first ticker fix measured 2.497 ns/call. Its profile exposed a separate
`Validate` call consuming 30.07% of sampled CPU. An intermediate guarded helper
measured 2.323 ns/call. Both were replaced by the smaller direct typed-identity
validation; the final profile marks both `Ticks` and `Validate` inline. The
final sample remains about 0.167 ns/call above the baseline, not an established
performance improvement. Every version measured zero timed allocations. The
memory profiles also include untimed testing/profiler/runtime allocations;
those totals are not per-call allocations. Both slower samples are retained.

The cgroup profile contains the final Hostfacts source. Later changes were
confined to Temporal's accessor and were profiled separately. No performance
claim is made for the cold diagnostic/error paths.

## Evidence retention

The follow-up manifest audits 96 execution records, 322 artifacts and 1,626
unique immutable source snapshots, with zero hash mismatches or source changes
during recorded runs. It includes failed attempts, CPU and memory profiles,
matching binaries, profile summaries, exact commands, exit status and final
source hashes. Raw artifacts remain ignored under
`hostfacts/testdata/test-upgrade-20260907`; only reports/manifests belong in the
eventual reviewed commit. Earlier upgrade evidence was preserved unchanged.
