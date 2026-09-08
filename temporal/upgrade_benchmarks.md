# Temporal benchmark comparison

Additional before/after profiles from the subsequent review fixes are recorded
in the [follow-up report](../hostfacts/review_followup.md). These original samples are retained.

Go 1.27.1, Darwin/arm64, Apple M1 Max. Each measured workload uses `-benchtime=30s -count=1 -cpu=1 -parallel=1`, with CPU profile, memory profile and test binary captured by that same invocation. `GOWORK=off`. Inputs and benchmark bodies are fixed between each compared pair.

One measured sample per workload per phase. These are observed means, not latency distributions or evidence of a stable timing trend. Unchanged controls moved materially: ParseDuration 112.3 → 70.05 ns and ParseRFC3339UTC 383.7 → 80.29 ns. Do not attribute those changes to this patch. Allocation reductions are the reliable result.

| Benchmark | Before ns/value | After ns/value | Before B/value | After B/value | Before allocs/value | After allocs/value |
| :-- | --: | --: | --: | --: | --: | --: |
| ParseDurationCanonical | 112.3 | 70.05 | 0 | 0 | 0 | 0 |
| ParseRFC3339Canonical | 137.9 | 90.71 | 0 | 0 | 0 | 0 |
| AggregateDurationDecimalMaximum | 526.4 | 343.1 | 48 | 48 | 1 | 1 |
| InstantJSONDecodeMaximum | 424.7 | 190.4 | 96 | 16 | 5 | 1 |
| DurationJSONDecodeMaximum | 422.1 | 230.1 | 96 | 16 | 5 | 1 |
| AggregateDurationJSONDecodeMaximum | 530.7 | 399.4 | 96 | 16 | 4 | 1 |
| NumericInstantJSONDecodeMaximumBatch | 138.7 | 56.1 | 24 | 0 | 1 | 0 |
| NumericDurationJSONDecodeMaximumBatch | 142.4 | 56.64 | 24 | 0 | 1 | 0 |
| NewInterval | 84 | 54.38 | 0 | 0 | 0 | 0 |
| WithTimeoutCancel | 448.9 | 363.9 | 272 | 272 | 4 | 4 |
| WaitZeroBatch | 343.6 | 10.94 | 248 | 0 | 3 | 0 |
| TickerOpenStop | 209.2 | 171.8 | 256 | 256 | 4 | 4 |
| ParseCompactUTC | 213.2 | 167.5 | 16 | 0 | 1 | 0 |
| ParseRFC3339UTC | 383.7 | 80.29 | 0 | 0 | 0 | 0 |

The three `Batch` benchmarks perform 64 calls per iteration to avoid Go’s iteration ceiling when operations become very fast. The table normalizes their bytes and allocations by 64 and uses the emitted `ns/value`. Original single-call baseline samples remain in the evidence; they are superseded for these comparisons, not erased.

The full optimized pass precedes the final compact-UTC allocation cleanup. Only `ParseCompactUTC` received a subsequent body change, replacing `Time.Format` with `Time.AppendFormat`; its row uses the subsequent focused measurement. The other follow-up change was the Wait documentation. Each profile remains attached to the exact source and measurement that produced it. Full tests, race checks and platform compilation were refreshed after that cleanup.

Profiles informed the changes:

- Replace temporary signed decimal strings with `strconv.AppendInt` into a fixed array.
- Replace generic JSON re-encoding during canonical checks with `jsontext.AppendQuote` into a bounded array.
- Validate zero waits without allocating a timer.
- Use `Time.AppendFormat` for compact UTC canonical validation.

Calendar parsing, canonical JSON quoting, numeric conversion, context cancellation and positive waits/tickers remain Go standard-library operations. Profiles show the final compact parser’s work in Go’s parsing and formatting functions, with no per-call allocation. `alloc_space` is sampled cumulative allocation, not peak resident memory.

All raw samples, iteration counts, effective durations, command arguments, power observations, source manifests and artifact digests are in [upgrade_evidence.json](upgrade_evidence.json). Raw profiles, binaries and logs remain under `testdata/test-upgrade-20260907/`. Short `1x` runs verified benchmark fixtures only and are not performance evidence.

Budget: the first full baseline requested 14 × 30 seconds; three corrected batch baselines requested another 3 × 30. The full optimized pass requested 14 × 30, followed by one compact parser refresh at 30 seconds. Effective execution durations, including calibration and profiling overhead, are preserved in each record.

## Timeout benchmark refresh during the full-module gate

NilAway required an explicit nil guard on the benchmark result after the timed
loop. A fresh invocation requested 30 seconds and captured both profiles plus
its matching binary: **377.3 ns/op, 272 B/op, 4 allocs/op**, 94,446,078 iterations
(35.635 effective timed seconds). The timed loop is unchanged. The original
before/after table above is preserved with its original source bindings.
This refresh does not establish a timing trend; its allocation result agrees
with the earlier candidate. The command, source manifest and artifact digests
are recorded as `temporal-timeout-refresh` in
[the Hostfacts/full-gate evidence](../hostfacts/upgrade_evidence.json).
