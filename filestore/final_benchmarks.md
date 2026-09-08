# Filestore benchmark evidence

After review, the user authorized deleting the retained binaries. Their hashes
and prior profile analyses remain in the records; CPU/memory profiles stay local.
See the [disposal receipt](../evidence/release-v2026.1.19/binary-cleanup.json).

Measurements use Go 1.27.1 on this Apple M1 Max. All accepted workloads ran serially with CPU=1 and a requested 30-second duration, with CPU/memory profiles and a retained binary from the same invocation. Raw artifacts stay local; JSON records contain commands, source snapshots, machine/power facts, counts and hashes. These are local author measurements, not independent release acceptance.

The complete attempt catalogue is [final-benchmark-catalogue.json](testdata/test-upgrade-20260907/final-benchmark-catalogue.json). It preserves every attempt, including the short validation microbenchmark that hit Go’s iteration cap and was replaced by a batch workload. Counts below are per operation, not resident memory.

## Matched production changes

| Workload | Before ns/op | After ns/op | Before → after B/op | Before → after allocs/op |
| --- | ---: | ---: | ---: | ---: |
| Bounded copy 128 B | 2094 | 174.1 | 32768 → 241 | 1 → 5 |
| Bounded copy 32 KiB | 5053 | 5127 | 32768 → 32881 | 1 → 5 |
| Bounded copy 1 MiB | 61490 | 61965 | 32768 → 32881 | 1 → 5 |
| ConfirmDurable | 29954 | 32252 | 704 → 944 | 11 → 14 |
| Touch | 4860883 | 4868367 | 898 → 1138 | 16 → 19 |
| OpenRoot | 36918 | 38610 | 424 → 528 | 8 → 10 |
| OpenDirectory | 14889 | 14748 | 432 → 656 | 5 → 6 |
| Ensure existing depth 1 | 9398298 | 9592481 | 1780 → 1604 | 24 → 16 |
| Ensure existing depth 16 | 16099612 | 11762199 | 51575 → 3669 | 1000 → 54 |
| Ensure/create/remove lifecycle | 11492946 | 12272313 | 6134 → 6262 | 65 → 69 |
| Two permission changes | 9390134 | 9447149 | 1269 → 948 | 24 → 16 |
| RemoveTree lifecycle | 5669840 | 5511045 | 1562 → 1794 | 16 → 19 |
| Stage/read/discard unwind | 11459325 | 11157228 | 2301 → 2301 | 36 → 36 |
| Read unwind | 15880 | 16120 | 688 → 688 | 11 → 11 |
| Unsupported sharing observation | 123.5 | 42.48 | 72 → 0 | 3 → 0 |
| Lexical Walk cancellation | 24153 | 24493 | 4264 → 4264 | 21 → 21 |
| Native Walk cancellation | 22700 | 23665 | 3112 → 3112 | 20 → 20 |
| Inspection validation batch (64) | 189.7 | 660.3 | 0 → 0 | 0 → 0 |

Most rows are single pairs and do not establish timing trends. Additional inode/custody checks intentionally cost allocations in several operations. Large copies retain bounded Go scratch space but add four small adapter allocations; the small-copy buffer reduction is the clear allocation improvement. Inspection schema validation became stricter and slower. The sharing result measures immediate unsupported-platform refusal on macOS, not Windows I/O.

Inspection’s direct native Lstat comparison has three observations per side: before 41,413 / 40,537 / 40,301 ns/op at 856 B and 12 allocations; after 2,200 / 2,197 / 2,196 ns/op at 320 B and 2 allocations. The median is 18.45 times faster for this exact owned regular-file workload. Its profiles explain the removed parent-handle acquisition. See [inspection-direct-benchmark-comparison.json](testdata/test-upgrade-20260907/inspection-direct-benchmark-comparison.json).

Walk’s original whole-sweep observations were lexical 21,994 and native 21,545 ns/op. Intermediate measurements were 29,058 and 27,034; the later cancellation pairs above were 24,153 → 24,493 and 22,700 → 23,665. Allocations stayed 4,264 B/21 and 3,112 B/20 throughout. Internal lexical directory readings were 20,636 → 32,999 ns/op with unchanged 3,768 B/12. These higher times remain visible; CPU samples are dominated by native calls and do not isolate a Primitive timing regression. No Walk speedup is claimed.

## Workloads added during the sweep

| Retained measurement | Benchmark | ns/op | B/op | allocs/op |
| --- | --- | ---: | ---: | ---: |
| checkpoint-native-handle-AppendExisting | BenchmarkNativeHandleAcquisition/AppendExisting | 16169 | 669 | 7 |
| checkpoint-native-handle-Lock | BenchmarkNativeHandleAcquisition/Lock | 35683 | 653 | 7 |
| checkpoint-native-handle-Read | BenchmarkNativeHandleAcquisition/Read | 15430 | 653 | 7 |
| checkpoint-native-handle-Rotate | BenchmarkRotateAppendOwnedHandleLifecycle | 10447563 | 1852 | 27 |
| checkpoint-native-handle-Update | BenchmarkNativeHandleAcquisition/Update | 23053 | 653 | 7 |
| checkpoint-parent-identity-IdentityPair | BenchmarkRootIdentityOwnedAndForeignDirectories | 7236 | 1196 | 13 |
| checkpoint-parent-identity-OpenParent | BenchmarkOpenParentNativeIdentityCustody | 45495 | 1304 | 18 |
| checkpoint-rename-CrossParent | BenchmarkRenameBinaryInodeRoundTrip/CrossParent | 20956819 | 4858 | 66 |
| checkpoint-rename-SameParent | BenchmarkRenameBinaryInodeRoundTrip/SameParent | 10292539 | 4197 | 48 |
| checkpoint-symbolic-observation-Canonicalize | BenchmarkCanonicalizeAncestorAndFinalLink | 33769 | 6256 | 61 |
| checkpoint-symbolic-observation-ReadLink | BenchmarkReadSymbolicLinkExactOpaqueTarget | 1453 | 160 | 4 |
| final-profile-held-standing | BenchmarkHeldStandingNativeIdentityTriad | 8314 | 1632 | 10 |
| final-profile-pipe-acquire | BenchmarkPipeAcquireBinaryEOFAndClose | 6026 | 208 | 4 |
| final-profile-pipe-held | BenchmarkPipeHeldBinaryTransfer | 947.7 | 0 | 0 |
| final-profile-stage-destination | BenchmarkStageDestinationCommitRemoveExactBytes | 23023123 | 3179 | 51 |
| final-profile-stage-lifecycle | BenchmarkStageReadDiscardExactBytes | 12074418 | 2301 | 36 |

The final-profile records refresh the five workloads whose EOF oracle changed to errors.Is during doctrine cleanup. Their earlier measurements remain in the catalogue; these are new observations, not code-only speedup comparisons. Other cleanup added ReportAllocs at parent benchmark entry points before setup/timing or improved failure diagnostics; their timed operations and data sizes remain those recorded.

See [upgrade_review.md](upgrade_review.md) for individual profile interpretations and [closeout_review.md](closeout_review.md) for the final verification scope.
