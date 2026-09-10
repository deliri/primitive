# Compass baseline — 2026-09-10

Base: Submission release v2026.1.45, commit 204e07d7cf9a9270a0d9a03e101b43de1823527d. Production was unchanged for both baseline runs. The upgraded benchmark harness was present; the getter case was added after the first CPU profile identified repeated name validation. The two passes cover disjoint cases.

Go 1.27.1, Linux/amd64, Furnace AMD EPYC 7282, GOWORK=off, GOMAXPROCS=8. One 30-second observation per case, CPU and memory profiles enabled, retained executable. Same server session; power posture was not independently controlled or recorded.

Workloads: embedded Current configuration; one fixed typed configuration through contiguous and one-byte readers; sixteen parses of a 128-byte ASCII name per batch; sixty-four String calls on that admitted name per batch. Exact results are checked in every timed iteration. The benchmark files have identical before/after hashes.

Full commands, iteration counts, source snapshots, stdout/stderr and profiles are retained under /home/d/engineering-evidence/primitive/compass-upgrade-20260910 in baseline-bench and baseline-string-bench. measurements.json records the absolute observations. [The comparison](after_benchmarks.md) includes all five cases.

The old unprofiled BenchmarkCurrent-10 number from an earlier machine/run is historical Git content, not a comparable baseline for this slice.
