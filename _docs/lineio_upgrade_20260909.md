> Historical initial sweep. [Current Lineio review follow-up](lineio_review_20260909.md) supersedes its source and verification status.

# Lineio fixed-memory streaming upgrade — review pending

This slice replaces whole-line Scanner accumulation with exact borrowed fragments
from Go's bufio.Reader.ReadSlice. A buffer window controls memory only; line and
stream extent are unrestricted by Primitive. Working memory is O(window size),
constant relative to stream extent; processing time is O(bytes processed).
No total-byte counter, growing aggregate, worker pool, runtime or product policy
was introduced. Lineio has no writer API; no new writer abstraction was invented.

## Contract and ownership

- Request owns Source and BufferBytes configuration. Positive native-representable
  buffer sizes are accepted; Go owns its minimum allocation. The former 16 MiB
  eager-allocation cap is removed along with line-length and total extent quotas.
- Reader owns one Go buffer; Capacity reports its actual fixed size.
- Fragment owns typed framing only: borrowed Bytes retain LF, CR and opaque bytes.
  More means the same line continues. Data must be consumed before its error.
- Clean EOF remains io.EOF. Native failures retain core.ErrLineIOScan and their
  native identity, including data-plus-error. Terminal calls never re-read.
- checkedReader validates native read counts and prevents accidental reuse of a
  caller's separately owned bufio.Reader buffer. Source ErrBufferFull is wrapped
  so it cannot masquerade as Go's internal full-window continuation sentinel.
- Source cleanup and cancellation remain caller-owned. Any deliberate complete
  product record materialization belongs to the consumer and is not claimed O(1).
- Scanner, BufferPolicy and MaximumLineBytes were removed. There is no shim.
  No production caller of Lineio exists elsewhere in this Primitive checkout.

Two narrow witness-lint sentinel-comparison waivers preserve the exact Go
ReadSlice/EOF distinction. They name ownership, the Go 1.27 reason, and reinspection
on the next Go upgrade. Using errors.Is for these two decisions would conflate a
wrapped source failure with a native control sentinel. Other error checks use
errors.Is / errors.AsType. The installed witness-lint passed.

The project-local testing_protocol.md was read completely before test edits.
This slice also amends the protocol with test/streaming-extent for **every
package, bidirectionally**, removing contradictory arbitrary byte-ceiling
requirements. This documentation amendment followed the Go verification runs;
all Go inputs remain identical. The separate streaming audit inventories all
63 packages; earlier upgrades are not exempt. Other packages remain unreviewed
under this new requirement.

## Hostile proof

The original published implementation was evaluated with a retained legacy-API
probe and refused valid 4 KiB and 8 KiB lines behind a 64-byte maximum. That red
probe was authored after the fragment design, then run against restored exact
legacy production; it is not presented as an earlier chronological test run.

The new source ErrBufferFull regression had three failing rows before the
production repair: empty, partial and exact-window data. The corrected suite
retains all three plus wrapped/native controls.

Tables exercise LF/CRLF boundaries, opaque bytes, exact/below/above windows,
long final lines, one-byte/chunked readers, data-plus-error, wrapped EOF,
cancellation/deadline identities, invalid read counts, no-progress readers,
nil/zero/typed-nil states, native configuration extremes, foreign buffer ownership,
terminal retry refusal and real Filestore handles. A generated 16 MiB + 1 line
continues through a 64 KiB window without materializing the fixture. Delimiter
permutations supplement named hostile rows. Fragment validation rejects empty
continuations and invalid delimiter placement.

439 passing test events, including parents and fuzz seeds; zero skips or
failures in the final ordinary race run. Statement coverage: 100%. This coverage
is supporting evidence, not a completeness or independent-acceptance claim.

Seven intentional production mutations caused real test failures; source was
restored after each run:

- lose-delimiter: 18 failing test events, including parents.
- refuse-full-window: 8 failing test events, including parents.
- lose-native-error: 6 failing test events, including parents.
- retry-terminal: 30 failing test events, including parents.
- reuse-source-buffer: 2 failing test events, including parents.
- source-error-as-continuation: 4 failing test events, including parents.
- invalid-empty-continuation: 2 failing test events, including parents.

External ingress inventory:
- New / Request.Validate configuration: FuzzRequestMemoryConfiguration.
- New / Reader.ReadFragment source bytes and chunk/window combinations:
  FuzzReaderFragmentConservation.
- Fragment.Validate is output-framing validation, tested against real producer
  output plus hostile invalid typed fragments. Reader lifecycle states are
  covered by exhaustive typed tables.

## Verification and scope

Base committed revision: 161e900507f2b2f79f4b1c8feca923057304f060
Toolchain: go version go1.27.1 linux/amd64
Exact uncommitted Go-input SHA-256: 84f5b2a2b269c8f28bd22132bc5fee2a8f9fdfbd9f8a0d8e2caa88e0c4851cc6

- verified-compile-darwin: exit 0; 0.642 seconds.
- verified-compile-windows: exit 0; 0.649 seconds.
- verified-final-complexity: exit 0; 0.003 seconds.
- verified-final-errcheck: exit 0; 0.162 seconds.
- verified-final-fix: exit 0; 0.15 seconds.
- verified-final-race: exit 0; 2.214 seconds.
- verified-final-staticcheck: exit 0; 0.431 seconds.
- verified-final-vet: exit 0; 0.131 seconds.
- verified-final-witness: exit 0; 0.008 seconds.

Linux executed. Darwin arm64 and Windows amd64 compiled only; neither platform
was executed here. No repository-wide gates were run. Ordinary tests used
-count=1, no selection filter, and no skips. Benchmark/fuzz -run=^$ filtering
was intentional and is retained in the command ledger. The inventory command
listed tests; it is not counted as another behavioral test run.

Fuzz targets ran serially after tools, tests, benchmarks and profile inspection;
each used 30 seconds, four workers, one-second minimization and a two-minute
timeout. Existing fuzz cache was not cleared; progress logs retain initial
corpus accounting. Inputs/secondary oracles have a finite 4 KiB test budget,
and allocation-configuration fuzzing validates native extremes without allocating
them. These are explicit test budgets, not production transfer quotas.

- fuzz-conservation: fuzz: elapsed: 30s, execs: 1358941 (0/sec), new interesting: 25 (total: 37)
- fuzz-memory-configuration: fuzz: elapsed: 30s, execs: 1744221 (0/sec), new interesting: 0 (total: 7)

All attempts are retained, including the original whole-line sweep, early lint
findings, the initial staticcheck inventory-marker finding, red probes and
mutations. Successful corrected runs do not erase failed attempts. The whole-line
sweep was superseded before its active fuzz phase; its seeds are not called a
completed fuzz campaign.

## Benchmarks and profiles

All seven final cases ran serially for 30 seconds each, one sample per case,
with benchmem, CPU profiles, heap profiles and matching test binaries retained.
GOMAXPROCS=8 on shared Furnace, AMD EPYC 7282, Linux amd64. No competing benchmark
or gate job was started by this work during these runs. Other machine activity,
CPU frequency and scheduler placement were not controlled. Memory profiles used
Go's 524288-byte sampling rate. Case fixtures are prepared outside b.Loop;
the measured work constructs the reader, consumes the complete stream, observes
byte/LF totals and checks terminal failure. Construction is measured separately.

Historical whole-line baseline:

| Workload | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| BenchmarkScanStreaming64Lines-8 | 4227.0 | 280 | 5 |
| BenchmarkScanStreaming4096Lines-8 | 118242.0 | 280 | 5 |
| BenchmarkScannerBoundary/ExactCRLFGrowth-8 | 974.9 | 424 | 10 |
| BenchmarkScannerBoundary/Final64KiBGrowth-8 | 81346.0 | 204954 | 16 |
| BenchmarkScannerBoundary/Final64KiBReserved-8 | 55623.0 | 139480 | 6 |
| BenchmarkScannerBoundary/Oversize64KiB-8 | 96331.0 | 205009 | 18 |
| BenchmarkScannerConstruction-8 | 385.3 | 232 | 4 |

Final fragment contract:

| Workload | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| BenchmarkScanStreaming64Lines-8 | 3262.0 | 248 | 5 |
| BenchmarkScanStreaming4096Lines-8 | 79483.0 | 248 | 5 |
| BenchmarkFragmentStreaming/Line64KiBBuffer64-8 | 38079.0 | 248 | 5 |
| BenchmarkFragmentStreaming/Line64KiBBuffer64KiB-8 | 28092.0 | 65720 | 5 |
| BenchmarkFragmentStreaming/Line1MiBBuffer64KiB-8 | 105405.0 | 65720 | 5 |
| BenchmarkFragmentStreaming/CRLFSplit64KiB-8 | 29840.0 | 65720 | 5 |
| BenchmarkReaderConstruction-8 | 338.9 | 200 | 4 |

Old Scanner results omit delimiters; the new contract preserves them and observes
raw byte totals. Whole-line growth/refusal cases also differ from fragment
continuation. These are **not identical-harness before/after statistics** and no
percentage speedup or significance claim is made. The manifest also retains the
superseded whole-line final measurements; they are not the current implementation.

With a 64-byte window, 64 lines, 4096 lines and a 64 KiB line each allocate
248 B/op in five allocations. With a 64 KiB window, increasing a line from
64 KiB to 1 MiB leaves allocation at 65720 B/op and five allocations. This
demonstrates constant allocation for the measured fixed-window workloads.
No 1 TB or 100 TB transfer was executed.

Short-line CPU samples attribute 75.62% cumulative time to Go's Reader.ReadSlice.
Fragment-workload allocation-space samples attribute 99.82% to Go's
NewReaderSize. The latter is cumulative allocation across millions of benchmark
iterations, **not resident memory**. The profiles justify retaining direct Go
buffering and byte search; no custom scanner, buffer pool or runtime was added.
Allocation churn includes intentionally constructing a new reader per operation.

## Evidence and review boundary

Machine-readable command, execution, source and artifact manifest:
lineio_streaming_evidence_20260909.json
Manifest SHA-256: 74c8d043f50d8943a4832f7ab5b1dbde0259b335bb569a006907f6feae2890aa

Raw current evidence: /home/d/engineering-evidence/primitive/lineio-streaming-20260909
Raw historical evidence: /home/d/engineering-evidence/primitive/lineio-upgrade-20260909

CPU/memory profiles and matching binaries are outside Git. Each retained run has
command arguments, source snapshots/digests, source patch, toolchain, exit status,
stdout/stderr and artifact identities. Profile extraction commands and installed
tool build identities are retained separately. Evidence binds this uncommitted
source by its full Go-input digest; the base commit alone is not claimed to
contain the change.

Witness's earlier 36-file mechanical caller-migration draft is incomplete and
uncommitted. Work on Witness stopped when the user took ownership of its source.
The drafts/snapshots and unrelated caller build failures remain recorded in
witness-handoff.json. They are not a Primitive completion gate. The installed
witness-lint still ran against Primitive and passed. Consumer release migration
must adopt the fragment API; this report does not claim Witness compiles.

No release bump, commit, push or independent acceptance was performed for this
slice. User review remains pending.
