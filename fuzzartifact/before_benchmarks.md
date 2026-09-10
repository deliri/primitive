# fuzzartifact before — 2026-09-10

The package was named `fuzzfinder` when these measurements were recorded.
The original commands, profiles and source identities remain unchanged.

Baseline production: ac10ac9c7edcb221ffb5668d45904219e00eafe3 (v2026.1.37).
Go 1.27.1, Linux amd64, AMD EPYC 7282, GOWORK=off, GOMAXPROCS=8.
Each case ran once with a 30-second benchmark budget on Furnace. CPU and memory
profiles, the binary, complete commands, source snapshots, and all attempt logs
are retained outside Git in
/home/d/engineering-evidence/primitive/fuzzfinder-upgrade-20260910/.

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| GeneratedName/corpus | 67.19 | 0 | 0 |
| GeneratedName/crasher | 66.56 | 0 | 0 |
| Find/128 native entries | 660852 | 50506 | 663 |
| Find/8192 native entries | 42014808 | 3033507 | 41867 |

Find scanned all entries but retained only the first 128 canonical names.
Directory construction was outside the timed operation. These are single-run
measurements on a shared host, not a distribution or independent acceptance.

The older report below describes a prior, narrower audit and is historical.

# fuzzfinder before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Find streams Filestore.Walk; retains at most 128 names (typed bound, not a hidden ceiling reservation of the tree).
