# AWS identity after measurements

These measurements used the strict-query/XML review build and temporal
integration through `proposed.overlay.json`. Those exact production files are
now applied, with equality recorded in `applied-source-equivalence.json` and
working-tree tests recorded in `applied-final-tests.json`.
Every workload has its own retained binary, CPU profile, heap profile, complete
command, output, source hashes, and effective-source overlay binding.

## Measurements

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Audience, 17 bytes | 22.43 | 33.15 | 0 | 0 | 0 | 0 |
| Maximum audience, 1,000 bytes | 220.8 | 57.78 | 0 | 0 | 0 | 0 |
| Exact signed request construction | 37,315 | 8,359 | 3,616 | 2,816 | 46 | 42 |
| Provider XML, one-byte token | 38,460 | 20,762 | 4,440 | 4,376 | 92 | 91 |
| Provider XML, maximum token | 483,065 | 469,300 | 69,836 | 69,771 | 101 | 100 |
| Bearer disclosure, one-byte token | 77.34 | 55.16 | 8 | 8 | 1 | 1 |
| Bearer disclosure, maximum token | 31,033 | 59,975 | 18,432 | 18,432 | 1 | 1 |
| Acquire, maximum token, transport seam | 499,249 | 398,065 | 126,549 | 126,085 | 156 | 153 |

“Before” for the added workloads means preserved original production running
the same new benchmark fixtures, as detailed in `before_benchmarks.md`. The
17-byte audience baseline is the actual pre-edit run of the original benchmark.
The ordinary and maximum audience implementations and bearer disclosure code
are unchanged; their large timing swings demonstrate why this table cannot be
read as a speedup comparison.

The machine was contended by other Go compilation/test/profile work and browser
processes. A process CPU snapshot is retained as
`concurrent-load-observation.json`. The benchmark commands themselves ran
serially. **No timing improvement or timing regression is established by these
samples.** Every sample is retained; no favorable attempt was selected.

All commands requested `-benchtime=30s -count=1 -p=1 -parallel=1` with both
`-cpuprofile` and `-memprofile` passed explicitly. Go's actual reported durations,
calibration overshoot, and iteration ceiling are retained in stdout. The
original audience baseline stopped at the billion-iteration ceiling before
30 seconds. The proposed audience-maximum and minimum-bearer samples reached
that ceiling after longer elapsed durations. Requested duration is not reported
as actual duration.

## Profile findings

- Request construction: `net/url.parseQuery` accounts for about two thirds of
  sampled allocation objects in both profiles. The baseline also attributes
  flat allocation objects directly to `net/url.ParseQuery`; that entry is absent
  in the proposed build. This is consistent with the reduced allocation counts
  after calling the error-returning Go API directly. It is profile evidence,
  not a claim that timing is isolated from compiler or runner effects.
- Maximum-token XML: Go's XML text decoding and byte-buffer operations are the
  main decoding work visible in the CPU profile. Runtime/OS activity is also
  prominent (`madvise`, `kevent`, and waits), limiting timing attribution under
  the observed contention. The byte ceiling remains enforced by Exchange before
  XML decoding; the tests prove the exact limit probe and body close.
- Acquisition memory: the sampled allocation-space profile attributes about
  40% cumulatively to Exchange's bounded copy path and about 55% cumulatively to
  Go's XML projection. These are overlapping call-tree views, not additive
  independent costs. The largest flat sites are Exchange's copy buffer, Go's
  byte-buffer growth, the bounded response storage, and XML value copies.
- Explicit bearer disclosure still allocates one string. At the maximum token
  extent the allocator charges 18,432 bytes. The returned header must own its
  exact token bytes; no fake zero-allocation claim is made.

The profiles identify future review surfaces; this pass does not replace Go's
query/XML parsers, bypass Exchange, or introduce a custom execution runtime.
The data is bounded, not constant-time: inspecting or disclosing N bytes takes
work proportional to N. The retained token and response ceilings bound aggregate
memory. The acquisition benchmark excludes real network latency, and the XML
benchmark excludes transport. Real TLS behavior is proved separately by tests.

## Temporal support measurements

| Workload | Before ns/op | After ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| Unchanged RFC3339 comparator | 151.0 | 127.3 | 0 | 0 |
| New compact UTC parser | Not present | 244.0 | 16 | 1 |
| New UTC-only RFC3339 parser | Not present | 122.5 | 0 | 0 |

The new APIs have no historical baseline. The existing RFC3339 benchmark fixture
is unchanged. These temporal results share the same contention limitation.

## Evidence locations

All artifacts are local under `testdata/test-upgrade-20260906`. There are 20
profiled executions: eight AWS baseline workloads, eight proposed AWS workloads,
and four temporal executions (one baseline and three after measurements). That
is 40 raw profiles and 20 benchmark binaries. `profile-plan.json` lists the
expanded phase, and the two initial baseline records identify the earlier runs.
`profile-analysis.json` accounts for all 60 successful `go tool pprof` analyses:
CPU, allocation-space, and allocation-object summaries for every execution.

Source snapshots and hashes bind each run to its actual files. One raw-XML fuzz
oracle was strengthened during the measurement sequence, and additional
byte/UTF-8 table rows were checked afterward. No production, benchmark
definition, or benchmark fixture changed for those test-only improvements.
Final package tests and sustained fuzzing use the stronger oracle. The earlier
profile's exact test-source snapshot remains retained rather than being relabeled
as a later tree.
