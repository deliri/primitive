# Primitive v2026.1.19

This release combines the reviewed Exchange continuation with the Filestore
upgrade. The sole version declaration is compass/config.json. User review approved the release and publication.

Exchange strengthens HTTP boundary validation, bounded receive/copy behavior,
replay/custody and error contracts while retaining Go net/http execution.
Its review and measurements are in [exchange/continuation_review.md](../exchange/continuation_review.md)
and [exchange/final_benchmarks.md](../exchange/final_benchmarks.md).

Filestore binds staging/recovery receipts to exact native facts, delegates
bounded copying to Go, closes unwind/cancellation gaps and improves native
inspection/directory acquisition. Tests use hostile tables and semantic fuzz
with real filesystem effects. See [filestore/closeout_review.md](../filestore/closeout_review.md)
and [filestore/final_benchmarks.md](../filestore/final_benchmarks.md).

The requested release gates passed with Go 1.27.1: go fix, vet, staticcheck,
deadcode, witness-lint, errcheck, nilaway, and goconst with the exact reasoned
four-group admission set. Goconst uses minimum length four, three occurrences,
and excludes tests. Runtime tests executed only Exchange and Filestore,
uncached: 9,384 and 5,515 passing Go events, zero failures/skips.
See the [gate report](../evidence/release-v2026.1.19/README.md) for exact commands,
source manifests, exceptions and all attempts.

Earlier Filestore tests measured 89.1% statement coverage and passed the race
suite. Linux and Windows test binaries compiled during the package sweep;
native runtime checks on those platforms are not claimed. These are local
author executions, not independent acceptance.

At the user's instruction, 212 reviewed test/benchmark binaries were deleted;
their hashes remain in the disposal report. CPU/memory profiles and large raw
logs stay local. Source, regression corpus and small evidence reports accompany
the release.
