# Primitive v2026.1.18

Filestore lexical walking now allocates for observed directory entries instead
of reserving the configured ceiling. Directory reads use the existing 64-entry
batch size and check cancellation between batches. Lexical ordering and refusal
before visitor delivery when the ceiling is exceeded remain unchanged. This
adds no API, dependency, or cgo requirement. Lexical sorting still retains the
actual entries of one directory up to its configured limit.

## Measured sparse-directory improvement

`BenchmarkLexicalSparseDirectory` opens, reads, checks, and closes a real
directory containing one regular file with the public maximum ceiling.
Published v2026.1.17 and the candidate used the same benchmark source,
Go 1.27.1, CGO_ENABLED=0, Apple M1 Max, and three serial 30-second samples.
CPU and allocation profiles, binaries, commands, source hashes, and all samples
are retained in `/private/tmp/primitive-walk-repair-36qcgoa9`.

| Sample | v2026.1.17 ns/op | v2026.1.18 ns/op |
| --- | ---: | ---: |
| 1 | 107722 | 21478 |
| 2 | 104765 | 20627 |
| 3 | 112334 | 21840 |

Median read time improved 5.02 times. Allocation fell from 1,059,385 to 3,768
bytes per operation (99.64%); allocation count remained 12. The baseline
allocation profile attributed 99.76% of allocated bytes directly to the lexical
reader. This is a sparse-directory measurement, not a whole-Hammer speed claim
or an O(1) memory claim for lexical sorting.

## Scoped validation

- Uncached full Filestore tests passed with CGO_ENABLED=0. The full race suite
  passed with `-parallel=4` using the shuffle seed from the failed resource run.
- Clean `CGO_ENABLED=0 go build -a ./...` passed; Compass release tests passed.
- Scoped vet, staticcheck, errcheck, witness-lint, and production complexity
  checks passed. Final equivalent fixture loop edits received a focused race
  rerun, staticcheck, and a fresh lexical fuzz campaign.
- Two serial 30-second fuzz campaigns covered lexical cardinality/refusal and
  directory replacement/skip standing. Both passed.
- Published production failed the sparse-capacity and cancellation regressions.
  Independent mutations dropping later batches and leaking over-limit prefixes
  failed exact-output/refusal assertions while controls passed.

Earlier attempts remain in the evidence directory: an incorrect closed-handle
test error assumption was repaired using the standard library's typed native
error; one concurrent race run exceeded the runtime's 10,000-thread limit; an
initial mutation run started before mutation installation and receives no
mutation credit. These are not counted as successful checks.

The isolated release source excludes concurrent unrelated test edits. Source
selection, test bindings, expected failures, and profile hashes are retained
beside the command receipts. This is local development evidence for this
allocation slice, not independent acceptance of every Filestore surface or the
remaining Hammer work.
