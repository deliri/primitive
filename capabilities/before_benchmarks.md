# Capabilities before measurements

The initial working-tree package run passed 142 test/subtest/fuzz-seed events
with no failures or skips, at 83.3% statement coverage. The original source,
untracked benchmark fixture, and prior reports were copied into
`testdata/test-upgrade-20260907/baseline-source` before changes.

Two benchmark fixtures already existed: `BenchmarkParseIdentity` and
`BenchmarkCatalogAll`. Their initial profiled executions happened before
production edits. Five additional workloads were then added and measured
against the still-original production: effect resolution, standard function
resolution, standard method resolution, unresolved-symbol resolution, and
classification JSON decoding with every allowed secondary owner.

Every invocation passed `-cpuprofile`, `-memprofile`, a retained `-o` binary,
`-benchmem -benchtime=30s -count=1 -p=1 -parallel=1`, and an exact single-workload
filter. Commands and actual output/durations are retained in the corresponding
JSON/stdout/stderr records. Requested duration is not substituted for actual
duration. No machine-wide isolation is claimed.

## Initial measurements

These historical samples are all retained. Test authoring continued during
this initial phase. The fixed-tree comparison described below is the primary
before/after comparison; no favorable historical sample is selected in its place.

| Execution label | Exact reported result |
| --- | --- |
| baseline-identity | `BenchmarkParseIdentity-10    	628525921	        47.75 ns/op	       0 B/op	       0 allocs/op` |
| baseline-catalog | `BenchmarkCatalogAll-10    	   90112	    373237 ns/op	       0 B/op	       0 allocs/op` |
| baseline-ResolveEffect | `BenchmarkResolveEffect-10    	   50292	   1154480 ns/op	       1 B/op	       0 allocs/op` |
| baseline-ResolveStandardFunction | `BenchmarkResolveStandardFunction-10    	 2250112	     18978 ns/op	   23761 B/op	      65 allocs/op` |
| baseline-ResolveStandardMethod | `BenchmarkResolveStandardMethod-10    	 1308751	     41094 ns/op	    8080 B/op	      58 allocs/op` |
| baseline-ResolveStandardUnresolved | `BenchmarkResolveStandardUnresolved-10    	 1656488	     39363 ns/op	   23760 B/op	      64 allocs/op` |
| baseline-ClassificationJSONMaximum | `BenchmarkClassificationJSONMaximum-10    	 1856072	     18219 ns/op	   9.88 MB/s	    1412 B/op	      44 allocs/op` |

## Frozen comparison baseline

The later `frozen-before-*` runs use `frozen-baseline.overlay.json`, selecting
all preserved original production files with the final frozen tests and the
same benchmark fixtures used for the after runs. They are reconstructed baseline
executions, not claims that these stronger tests existed before the upgrade.
The new regression cases fail against original production; benchmark commands
select no tests and only the declared valid workload.

This additional baseline resolves the evolving test-tree limitation of the
initial phase. The original seven executions remain visible as earlier
observations. The frozen identity run reached Go's billion-iteration ceiling
at 17.524 seconds of reported package duration despite requesting 30 seconds.
Its measured result was 17.01 ns/op; the iteration ceiling is an explicit budget
divergence, not a full 30-second measurement.

The final comparison table and profile interpretation are in
`after_benchmarks.md`. Source snapshots, exact command arguments, full outputs,
binaries, profiles, profile analyses and the final artifact manifest stay in
this package's local evidence bundle.
