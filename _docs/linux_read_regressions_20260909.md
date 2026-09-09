# Linux regression repair — reviewed 2026-09-09

**Reviewed**: 2026-09-09
**Mode**: local uncommitted changes
**Host**: `d@192.168.1.81:/home/d/code/primitive`
**Base**: `c7dcf14acc6d7d6633fe335c96a24366421dceda` (v2026.1.25)
**Decision**: APPROVE with comments
**Issue counts**: 0 bugs, 1 suggestion, 1 nit

The structured review below is the verdict. The author's execution notes and
the companion evidence JSON remain attached so the review is not detached from
the Linux race, fuzz, and benchmark record.

## Summary

This slice correctly abandons the GCS `RemainingBytes` lie: both public read doors (`streamGCSRead` and `streamGCSListedRead`) now copy through `io.LimitReader` and prove exhaustion with a one-byte `io.ReadFull` on the unbounded SDK reader, so a matching prefix can no longer certify success while extra provider bytes remain (including empty metadata hiding a body). Error ownership is sound: `gcsReadSource` keeps the native SDK error independent of `io.Copy`'s return, bare EOF is the only graceful completion (same unwrapped-EOF rule as ExactReader/filestore, under expiring Witness waivers), non-EOF source failures stay `Source`+`Integrity` even beside a destination error, and destination failures are not labeled source when the reader only saw nil or bare EOF. Named-read failures still abandon the stage; listed-read failures still leave a bounded prefix in the caller writer (pre-existing `io.Writer` contract, and the new tables pin that extra bytes never land). Residual risk is intentional: `copyGCSExact` no longer applies ExactReader's empty-read / invalid-count guards, which is acceptable because production only passes `*storage.Reader`.

## Issues

### Issue 1 -- Severity: suggestion
- File: gcsobjects/client.go:1062
- Description: The `copyGCSExact` comment restates the implementation (stdlib copy, one scratch byte, extra bytes never reach the destination) instead of the non-obvious WHY. The load-bearing constraint is that the EOF probe must run on the unbounded reader: `io.LimitReader` reports EOF at the metadata ceiling without reading further, which is the same class of false completion the old `gcsExactSource.RemainingBytes` wrapper had.
- Suggestion: Keep one WHY sentence: metadata length is an expectation, so the probe has to use the raw SDK reader rather than the limited copy source.
- Status: open

### Issue 2 -- Severity: nit
- File: runnercontrol/subject_run_unix_external_test.go:9
- Description: `gofmt` will rewrite this import block: `filestore` was inserted into the standard-library group. The new GCS test files have the same mix (`gcsobjects/read_stream_regression_test.go:7`, `gcsobjects/read_stream_fuzz_test.go:6`, `gcsobjects/read_benchmark_test.go:5`).
- Suggestion: Run `gofmt` on the touched test files before commit.
- Status: open



## Author follow-up — both comments resolved

The review above is preserved as received. Issue 1 is resolved by explaining
that the EOF probe must use the raw SDK reader: a limited reader can report EOF
at the expected length without proving provider exhaustion. Issue 2 is resolved
by separating standard-library and Primitive imports in the four named tests
and applying gofmt. gofmt preserves existing import groups, so the grouping was
fixed explicitly.

Validation: gofmt reports no remaining changes; GCS Witness passes with the same
two EOF waivers. These edits change comments and import grouping only. Existing
race, fuzz and benchmark evidence retains its original source coordinates; no
new runtime or performance result is claimed. The exact follow-up diff, hashes
and check records are in `linux_read_regressions_20260909_review_followup.json`.

---

# Slice notes (author, pre-review)

Base: `c7dcf14acc6d7d6633fe335c96a24366421dceda` (v2026.1.25).
Candidate: uncommitted changes in `/home/d/code/primitive` on `d@192.168.1.81`.
The full 2,214-line project testing protocol was read before test edits. Exact
commands, source hashes, source snapshots, failed attempts, outputs, profiles
and matching benchmark binaries are indexed in the companion evidence JSON.
This is local execution evidence; independent acceptance has not been issued.

## Changes

GCSobjects previously derived remaining source bytes from the expected metadata
length. That could certify a matching prefix while the provider still had an
extra byte, including when metadata claimed an empty object. Both public read
paths now use `io.Copy` with `io.LimitReader`, then `io.ReadFull` into one byte
of scratch to prove EOF. The scratch byte never reaches the destination.
A small read-error carrier preserves the difference between SDK and destination
failures. Bare EOF is distinguished from joined failures under two narrow,
expiring Witness waivers. No custom transport, scheduler or hashing algorithm
was introduced. Copying stays bounded by Go's standard buffer allocation.

Deploy and Distribution fixtures now include Release's required explicit empty
build-tag selector collection. Release validation was kept strict. These two
fixtures still enter through the public provenance JSON decoder; this slice
does not redesign their historical fixture construction.

GCS zero-length validation fixtures now carry actual empty digests. Client
lifecycle and retry tests use a test-owned credential file and local token
endpoint, so they no longer depend on workstation Google credentials. All new
filesystem writes use Filestore.

Runnercontrol's cancellation fixture now owns a supervisor script that ignores
supervisor arguments and emits output until the owned process group is
cancelled. GNU `yes` rejected those arguments before emitting output. The test
still requires cancellation identity, stop-controller failure and reaped
process evidence; it uses no timing sleep.

## Hostile proof

The original clean Linux run failed Deploy, Distribution, GCSobjects and
Runnercontrol. After fixture repair, three SDK read cases remained red:
extra body bytes, chunked extra bytes and a nonempty body behind empty metadata.
Those now pass. A real list-to-read table covers the second public caller.
Direct tables pin short reads, extra bytes, empty streams, joined EOF,
cancellation, source failure beside final bytes, destination failure beside
complete writes, and refusal ownership. Red tests also caught two mistakes in
the first candidate's handling of errors at exact byte counts; both were fixed
before this checkpoint. They are not represented as pre-existing regressions.

A deliberate mutation removing the EOF proof fails the listed-read table and
public fuzz seeds. Its source overlay and failed output remain in the evidence.
`FuzzGCSReadExtentSemanticBoundary` executes both public read doors through the
real SDK and local HTTP providers. It compares exact bytes, preserves typed
refusals, requires zero returned proof on failure and checks abandoned stages.
Its opaque payload budget is 4,096 input bytes plus one mutation byte.

## Validation

- Full Linux race command: `GOWORK=off GOMAXPROCS=8 go test -json -race -count=1 -p=4 -parallel=8 -timeout=20m ./...`.
- **61 packages passed**, exit 0, 48.77 seconds. Four tests were skipped: three
  authenticated live GCS tests and the Release Darwin/ARM64 host-specific check.
- Raw test events: 52,100 pass, 4 skip, 7 fail. The seven fail events are deliberate
  Testserial child-test diagnostics; their owning package passed. Event counts
  are not counts of independent tests or defects.
- Scoped vet, staticcheck v0.8.0 and errcheck v1.20.0 passed for all four packages.
- GCS production gocyclo v0.6.0 passed at <=10. Pinned Witness passed with the two
  EOF waivers. Initial private-module installation failure and the missing parent
  `ReportAllocs` finding remain visible, followed by their resolved attempts.
- Fuzz: requested 30 seconds, 4 workers, **7,550 executions**, no failures. Package
  elapsed was 31.110 seconds with continuous progress; whole-command elapsed was
  94.98 seconds, including work outside the package test interval.
- The full race run preceded the parent benchmark's `ReportAllocs` addition only.
  Production, ordinary tests and fuzz callbacks match the final measured candidate;
  Witness and both final benchmark compilations include that final addition.

This was a Linux execution. macOS/Windows execution and a complete module-wide
analyzer gate were not run in this slice. The slice closes the observed Linux
failures; it does not declare a complete GCSobjects package sweep.

## Benchmarks and profiles

The final comparison uses byte-identical benchmark/provider harness files in a
Git worktree at the released base and the candidate checkout. Both use Go1.27.1,
GOMAXPROCS=8, serial 30-second measurements, fixed nonuniform payloads, actual SDK
HTTP reads, integrity verification, filesystem staging and discard. This is a
local provider benchmark, not remote GCS latency. Both cases share one CPU and
one memory profile per phase; matching binaries and complete logs are retained.

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 KiB | 737,666 | 710,403 | 75,714 | 42,225 | 505 | 505 |
| 1 MiB | 6,683,688 | 6,292,483 | 73,247 | 73,337 | 502 | 504 |

These are samples, not a statistically established speedup. Other server jobs
were observed; host isolation and identical power/load posture were not proven.
All earlier attempts remain: the first large-response fixture accidentally used
chunked framing and triggered SDK logging; later attempts corrected framing and
then the parent allocation-reporting flag. They are not substituted for the final
identical-harness pair.

CPU profiles remain dominated by Go syscalls and hardware-backed SHA-256. Memory
profiles show lower copying-buffer allocation for the small-object workload.
Passing the explicit limit directly to Go lets `io.Copy` size its buffer to that
limit. Large-stream memory remains approximately constant; the stronger final
read has a small allocation cost. Profiles cover setup and both workloads with
different iteration counts, so their aggregate percentages are not per-operation
allocation comparisons.

Raw evidence: `/home/d/engineering-evidence/primitive/linux-repair`.
The original clean failure run remains in the sibling
`v2026.1.25-linux-race-20260909T012203Z` directory. Historical tracked sources were
reconstructed from retained patches and the base commit where necessary, then
checked against their original execution hashes. The source snapshot audit has
no missing entries. Large raw artifacts remain outside the repository.

Review this slice before a version bump, commit or push.
