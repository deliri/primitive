# Hostfacts profiled benchmark comparison

Additional before/after profiles from the subsequent review fixes are recorded
in the [follow-up report](review_followup.md). These original samples are retained.

Go 1.27.1, Darwin/arm64, Apple M1 Max, `GOWORK=off`. Each workload requests 30 seconds with `-count=1 -cpu=1 -parallel=1 -benchmem`; the same invocation writes CPU and memory profiles and its matching test binary. Benchmark timing ran without concurrent agent test/analyzer/fuzz campaigns.

One sample per workload per phase. These are observed means, not percentile distributions or a stable latency trend. Unchanged controls expose machine/run variability; do not attribute their timing changes to production improvements.

| Workload | Before ns/value | After ns/value | Before B/value | After B/value | Before allocs/value | After allocs/value |
| :-- | --: | --: | --: | --: | --: | --: |
| GoOOMBannerChunkBoundary | 257887 | 11690 | 41008 | 41008 | 2 | 2 |
| MountInfoMatching | 3114 | 361.5 | 24 | 24 | 2 | 2 |
| CgroupMembershipMatching | 99.52 | 66.31 | 96 | 16 | 2 | 1 |
| GoOOMStateJSON | 153.8 | 78.12 | 32 | 16 | 2 | 1 |
| GoOOMEvidenceJSON | 1602 | 1541 | 952 | 944 | 27 | 26 |
| MemoryTriggerMaximumBatch | 12.6781 | 8.73594 | 0 | 0 | 0 | 0 |
| ClassifyGoOOMBanner1KiB | 14573 | 4158 | 41008 | 41008 | 2 | 2 |
| ClassifyGoOOMBanner1MiB | 5.76734e+06 | 62451 | 41010 | 41008 | 2 | 2 |
| CgroupMountInfoStreaming1KiB | 13754 | 9482 | 69704 | 69704 | 5 | 5 |
| CgroupMountInfoStreaming1MiB | 2.90674e+06 | 2.40369e+06 | 69705 | 69710 | 5 | 5 |
| ReadBoundedValueSparse | 1423 | 912.2 | 4144 | 4144 | 2 | 2 |

The maximum-memory-trigger workload performs 64 checked calls per iteration; this table normalizes by 64. Its original single-call baseline hit Go’s one-billion-iteration ceiling after 8.856 effective seconds and is superseded by the 30-second-requested batch baseline. Both attempts remain recorded. All 11 paired benchmark function bodies match exactly. The streaming mount helper’s failure diagnostic was clarified for Witness after baseline; its input, timed operations and checks are unchanged.

The full baseline requested 11 × 30 seconds, followed by a corrected batch baseline at 30 seconds. The full candidate requests 11 × 30 seconds. Calibration/profiling overhead, iteration counts and effective timed durations are recorded per command. `1x` fixture runs are not timing evidence.

Profiles informed these specific changes:

- Replace the custom per-byte OOM banner matcher with Go `bytes.Contains` and a fixed overlap between 32 KiB reads. Keep reading the entire declared extent after a match.
- Replace allocating cgroup `bytes.SplitN` with two `bytes.Cut` calls.
- Avoid a 64 KiB decoding buffer for valid unescaped mount paths; escaped input retains the bounded decoder.
- Validate canonical OOM JSON with `strconv.AppendQuote` into a fixed array.

The original combined CPU profile attributed 16.16% cumulative CPU to the custom banner matcher and 8.42% flat CPU to mount-path decoding. The final line profile attributes 11.62 seconds to the two Go search calls and 0.04 seconds to overlap copying across this combined run. The new implementation delegates byte searching to Go. Scanning remains O(n) time with fixed auxiliary memory; it is not O(1) time for arbitrary input. Bounded cgroup reads retain their existing 1 MiB admission ceiling and incremental allocation.

CPU percentages span different workloads and iteration counts, so they are not directly comparable per operation. Memory `alloc_space` is sampled cumulative allocation, not peak resident memory. Raw profile totals in gigabytes are not the process’s live footprint. The table uses Go’s per-operation allocation measurements.

See [upgrade_evidence.json](upgrade_evidence.json) for all samples, commands, exact source snapshots, tool/machine/power observations and artifact hashes. Raw logs, binaries, CPU/memory profiles and snapshots stay ignored under `testdata/test-upgrade-20260907/`; the reports are the reviewable repository artifacts.
