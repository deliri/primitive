# fuzzartifact after — 2026-09-10

Find now streams every matching name to a synchronous visitor. The result owns
only accounting; there is no retained-name array, selection, sorting, or total
entry quota. Filestore and Go continue to own directory I/O. Exact Go-generated
filename syntax remains validation. ArtifactKind JSON no longer rejects a valid
token because surrounding whitespace pushes its document beyond 64 bytes.

| Case | Before ns/op | After ns/op | Before B/op | After B/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| GeneratedName/corpus | 67.19 | 68.23 | 0 | 0 | 0 |
| GeneratedName/crasher | 66.56 | 68.19 | 0 | 0 | 0 |
| Find/128 native entries | 660852 | 620170 | 50506 | 47837 | 663 |
| Find/8192 native entries | 42014808 | 38903142 | 3033507 | 3030893 | 41867 |

Both phases use the same benchmark names, native directory cardinalities,
Go 1.27.1 Linux amd64 host and GOMAXPROCS=8, and 30 seconds per case. The changed
output contract is explicit: before selected at most 128 names; after delivers
all names. These are single runs, not a claimed performance trend.

CPU and allocation profiles show that native Go directory reads and entry
metadata dominate Find. Removing retained-name storage saves roughly 2.6 KiB per
call. B/op is cumulative allocation during enumeration and grows with entry
count; it is not the amount of simultaneously retained memory. Primitive keeps
constant-size accounting and uses Filestore's fixed directory batches.

Evidence: /home/d/engineering-evidence/primitive/fuzzfinder-upgrade-20260910/
on Furnace. Each run records its base commit and exact dirty source hashes.
CPU/memory profiles and binaries are under baseline-profiles and after-profiles.
The manifest preserves every attempt, including the red cases and mutations.

Scoped race tests, go fix -diff, vet, staticcheck, errcheck, witness-lint and
gocyclo <= 10 pass. Darwin arm64 and Windows amd64 test binaries compile;
only Linux executed. Four semantic fuzz targets each passed a 30-second
campaign. Five compiled production mutations were rejected. Full-module gates
were not run. This is author-run evidence pending user review, not independent
acceptance. These campaigns preceded the clean package rename described below.

Review follow-up: `/tmp/fuzzfinder_review_20260910_findings.md` found no production
bug. The package is now `fuzzartifact`, with Core's PackageFuzzArtifact and
ErrFuzzArtifact identities; the former package and symbols are removed. Current
Primitive catalog entries and claims use the new name. Architecture role labels
now describe constant-size accounting and synchronous enumeration. JSON decoding
explicitly takes caller-owned whole input, and Visit documents its directory
lifetime and caller-owned cancellation. No competing parser, callback goroutine,
or new scan state was introduced. The apparent format-error wrapping difference
already preserves identical errors.Is membership through Core's parent relation.

After rename, uncached package race tests, focused Core catalog/error/ownership
tests, go fix -diff (empty), vet, staticcheck, errcheck, witness-lint and package
complexity checks passed. All Primitive production packages and _hammer/claims
built. Darwin arm64 and Windows amd64 test binaries compiled; execution remained
Linux-only. These checks are recorded as rename-* runs against c27023b and exact
dirty source snapshots. Benchmark and fuzz campaigns were not repeated for this
mechanical rename; their original source identities and profiles are preserved.
Final subsequent edits affect comments and these navigation/evidence documents.

The older report below is retained as historical evidence.

# fuzzfinder after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./fuzzfinder`

```
BenchmarkCacheFormatGeneratedName/fuzz-corpus-10         	45184513	        26.57 ns/op	       0 B/op	       0 allocs/op
BenchmarkCacheFormatGeneratedName/fuzz-crasher-10        	46953867	        26.24 ns/op	       0 B/op	       0 allocs/op
BenchmarkFindRealDirectory128-10                         	    4525	    259791 ns/op	   53969 B/op	     669 allocs/op
BenchmarkFindRealDirectory8192-10                        	      54	  20592608 ns/op	 3181825 B/op	   42123 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
