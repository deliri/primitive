# Capabilities after measurements

The comparison uses preserved original production through
`frozen-baseline.overlay.json` and applied production directly. Both sides use
the same frozen tests and benchmark fixtures. All seven workloads run once per
side, serially. The final after phase follows `commit-source-tests.json`.
Every invocation explicitly captures a CPU profile, memory profile and retained binary. There are no parallel
benchmark workloads. The original seven preliminary measurements and the seven
intermediate after measurements are also retained. A final constant-only correction gives the
shared architecture error context one owner; the final after profiles use that
exact production source.

Toolchain: Go 1.27.1, darwin/arm64, Apple M1 Max. The observed benchmark suffix
is `-10`; no explicit GOMAXPROCS override was set. The workspace file and process
runtime settings are recorded in the local environment evidence.

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Parse identity, filesystem | 17.01 | 17.17 | 0 | 0 | 0 | 0 |
| Complete catalog construction | 77486 | 2470 | 0 | 0 | 0 | 0 |
| Resolve transport effect | 156361 | 8664 | 0 | 0 | 0 | 0 |
| Resolve composite HTTP function | 8290 | 9058 | 23761 | 23761 | 65 | 65 |
| Resolve command execution method | 19703 | 13402 | 8080 | 8080 | 58 | 58 |
| Resolve unlisted HTTP symbol | 19318 | 7662 | 23760 | 23760 | 64 | 64 |
| Decode all ten owners, 180 JSON bytes | 13878 | 4353 | 1412 | 1411 | 44 | 44 |

## Interpretation and limits

These timings are observations on a contended machine. Browser renderers,
indexing, security scanning and another Go test process were observed during
measurement; `concurrent-load-observation.txt` retains a process snapshot.
Unchanged identity parsing varied substantially. **No isolated timing speedup
or timing regression is established.** No favorable run was selected in place
of an unfavorable one. Allocation counts for the seven workloads were unchanged.
The one-byte JSON B/op difference is retained without claiming an allocation
optimization in unchanged JSON code.

The catalog change removes repeated work identified by the original CPU
profile: one `Catalog.Validate` previously revalidated the entire core catalog
for every derived capability. The applied implementation validates the input
catalog once and compares its entries directly with core's authority. Exact
membership, role coverage, effect owners and invalid catalog refusal remain
checked. In the final after catalog profile, `capabilityFor` disappears from
the reported hot path. Direct architecture lookup accounts for about 42% of
sampled CPU and the remaining whole-input validation about 43%. These percentages
describe profile composition, not isolated per-operation speed. The profile is
`commit-after-catalog.cpu.pprof`; its analysis and binary are retained beside it.

Symbol lookup still allocates its rule tables. The initial function allocation
profile attributes about 80% of objects to `effectSymbolRules` and about 19% to
`purePackageRules`; the method profile attributes virtually all sampled objects
to `standardMethodRules`. This pass does not claim those allocations were
eliminated. The extra effect/replacement validation strengthens correctness.

All calls requested `-benchtime=30s -count=1 -p=1 -parallel=1`, one exact benchmark
filter, `-benchmem`, `-cpuprofile`, `-memprofile`, and `-o`. Requested time is not
actual duration: calibration can overshoot, and both frozen-before and
final-after identity runs
hit Go's billion-iteration ceiling before 30 seconds (17.524 and 17.565
seconds of reported package duration, respectively). Every actual duration and
iteration count remains in stdout and the execution record.

The JSON workload has maximum secondary cardinality and a 180-byte canonical
source. The 1,024-byte input ceiling is separately proved by tests at one below,
exactly at, and one above the boundary. Each timed operation retains its result
and checks the declared result afterward. No transport or live external service
is part of these catalog measurements.

## Retained evidence

All artifacts are local under `testdata/test-upgrade-20260907`: 28 profiled
executions, 56 raw CPU/memory profiles, 28 benchmark binaries, exact commands,
stdout/stderr, source snapshots, overlays and hashes. The inventory also covers
nine fuzz binaries and the mutation evidence. Profiles and binaries are ignored
local artifacts; the package reports are committed.

`final-phase-plan.json` declares the frozen before/intermediate-after and fuzz
sequence; `commit-profile-plan.json` declares the final-source after captures. `profile-analysis.json` records CPU, allocation-space and
allocation-object analyses. `manifest.json` inventories the final bundle and
binds these reports; `artifact-audit.json` independently walks that local bundle
for exact file membership and hashes. This is a local consistency check, not an
independent acceptance receipt. Repository gates remain deferred.
