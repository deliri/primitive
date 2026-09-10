# Runnercontrol coverage streaming — 2026-09-10

Release v2026.1.52, based on f877b3a. This closes the coverage reader's input
extent and accumulation slice; runnercontrol as a whole remains open.

## Mechanical change

GoCoverageCompiler no longer retains a line or source location. It incrementally
recognizes the existing whitespace-separated fields, folds decimal counts with
native uint32/uint64 overflow checks, and keeps only scalar state, a 12-byte
closed mode spelling, and at most four UTF-8 bytes. Unicode separators remain
valid across writes. ASCII source-location bytes take a direct path; they are
not copied or interpreted as product identity. There is no goroutine, global
state, competing I/O runtime, or added workflow.

Removed GoCoverageLineMaximumBytes and GoCoverageLineMaximum. These arbitrary
line/record quotas were not working-memory budgets. The 12-byte header storage
is the longest admitted Go mode spelling, not a stream quota. Numeric fields
accept arbitrarily many leading zeros without retaining them. Totals are uint64;
native overflow remains a typed refusal. Basis-point calculation uses math/bits
wide multiplication and division, so multiplying by 10000 does not impose a
smaller hidden statement ceiling.

Write remains a direct io.Writer: a newly detected semantic refusal permits the
current raw chunk to be retained, Seal returns zero observation plus the typed
refusal, and subsequent Write calls expose the stored error. Cancellation and
source I/O remain owned by the caller, with no internal workers to shut down.
The compiler does not own evidence files, manifests, or product completion policy.

## Evidence and scope

All runs occurred by SSH on furnace under /work/code/primitive, Go 1.27.1 linux
amd64. Evidence directory:
/work/engineering-evidence/primitive/runnercontrol-coverage-20260910.
Every run retains exact argv, committed base and dirty-tree facts, full source
snapshots, toolchain/environment, output hashes and bytes, exit, and source
stability. Failed runs are retained, never overwritten. These author-run facts
are not independent acceptance receipts.

Three tests failed against unchanged production: an 8 MiB source location,
1,048,832 records, and a two-MiB leading-zero execution count. They now pass using
repeated fixed-size chunks. These are finite continuation checks, not a claim
that a terabyte profile was executed. A structural guard refuses input-sized
fields in the compiler; the maximum-total tests directly exercise arithmetic
state and do not claim to stream uint64-max records.

The arithmetic mutation discarded the high multiplication word; two native
boundary tests failed. The mutation was restored and its identity and failed run
remain in evidence. Existing obsolete line-quota tests were replaced with native
overflow and split-Unicode boundaries.

A real installed-Go subprocess creates a one-statement profile from a temporary
module. The test feeds the emitted profile one byte at a time and checks exact
counts. That emitted profile also seeds the existing semantic fuzz target. Its
callback now compares whole-input and 1/7/4096-byte fragment results, including
typed overflow identity, in addition to validation and arithmetic conservation.
This is real toolchain-to-parser proof; it is not downstream manifest/ledger proof.

The package passed tests and two race/shuffle runs. Vet, staticcheck, witness-lint,
module build, and the final 30-second four-worker coverage fuzz phase are recorded
individually. Strict errcheck's 19 earlier findings in unchanged test files remain
open. Initial lint failure on the benchmark parent and staticcheck's discovery of
deprecated runtime.GOROOT in the preceding metadata test are retained; both were
fixed. The subprocess now locates Go through the runner's explicit PATH.

## Benchmark observations

Fixed inputs: one coverage record with 1 KiB or 64 KiB source location. Same
machine, toolchain, -cpu=1, 30-second benchmark duration, one pass per source phase.
The result is observed and checked on every iteration. CPU and memory profiles
and their matching binaries are retained for each phase. Baseline/candidate runs
have different source fingerprints as expected; test-only additions do not
change the declared workload. One observation per case per phase is not a
distribution or a statistically established performance improvement.

The first candidate removed allocation but slowed parsing, motivating an ASCII
location fast path. Both candidate phases remain in the evidence. Benchmark
stdout below preserves absolute times and allocation figures.

streaming-before:

```text
BenchmarkGoCoverageStreaming/one_KiB_location         	12269330	      2932 ns/op	 357.81 MB/s	    2448 B/op	       6 allocs/op
BenchmarkGoCoverageStreaming/64_KiB_location          	  226645	    157486 ns/op	 416.30 MB/s	  147600 B/op	       6 allocs/op
```

streaming-after:

```text
BenchmarkGoCoverageStreaming/one_KiB_location         	 5411373	      6657 ns/op	 157.59 MB/s	       0 B/op	       0 allocs/op
BenchmarkGoCoverageStreaming/64_KiB_location          	   86953	    413038 ns/op	 158.73 MB/s	       0 B/op	       0 allocs/op
```

streaming-final-benchmark:

```text
BenchmarkGoCoverageStreaming/one_KiB_location         	12972632	      2783 ns/op	 376.90 MB/s	       0 B/op	       0 allocs/op
BenchmarkGoCoverageStreaming/64_KiB_location          	  218080	    165100 ns/op	 397.10 MB/s	       0 B/op	       0 allocs/op
```

## Next surfaces

Go test JSON line buffering and build-event accounting, JUnit/profile ingress,
seal lifecycle and producer/accounting consistency remain. This slice does not
close the package-wide hostile-matrix review, the remaining ingress inventory, or
the strict errcheck baseline. Primitive continues to own blind typed mechanics;
the consuming product owns what the resulting observations mean.
