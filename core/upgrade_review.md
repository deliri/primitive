Core now enforces six previously broken mechanical boundaries while keeping execution in Go. The recorded before run contains 39 failing leaf cases across six defect families; those are not 39 independent bugs.

| Boundary | Before | Retained change |
| --- | --- | --- |
| Digest extent | The internal unsigned count could cross the signed size domain and continue hashing. | Refuse before hashing, preserve count/hash state, latch the refusal until Reset. |
| Reader outcomes | Invalid counts and over-limit proof bytes discarded a simultaneous reader error. | Return the Core refusal with the original reader cause. |
| HTTP endpoint | An admitted URL could expand beyond its own bound when Go escaped it. | Use the fixed %HH maximum expansion to admit provably short input; check Go's exact canonical projection for larger input before publication. |
| Path ingress | Go cleaning could erase NUL, malformed UTF-8, or excessive raw runes before validation. | Validate raw text before delegating lexical resolution to filepath. |
| Numeric JSON | Over-width integers copied attacker-sized strings into strconv parsing/error state. | Check the uint64 decimal width before passing bounded input to strconv. |
| JSON projection | A caller's no-op semantic validator could waive Core's structural limits. | Core enforces document, depth, field, array and folded-uniqueness limits before the caller callback. |

The first corrected endpoint benchmark exposed extra canonical-string work. Its escaped-path memory profile attributed 62.14% of allocated bytes to URL.String. The retained guard uses the fixed three-byte percent spelling only as a conservative upper bound; it contains no encoder or parser. Near the bound it still calls Go's String for the exact answer. The installed Go URL source is hash-bound in the evidence. An unsafe two-byte-estimate mutation was killed by three oversized-publication rows. Intermediate timings remain in the report.

The release-version baseline CPU and allocation profiles showed repeated decimal formatting and parsing during validation. Its private uint32 fields already admit every constructed triple, so Validate now checks the construction bit. Text syntax remains enforced at ingress. The unset-value mutation was killed by the tests.

Digest tables compare exact bytes/counts with Go SHA-256 across stream boundaries, sealing, refusal, reset, and io.Copy/io.MultiWriter composition. The reset mutation was killed. Secret-copy tables exercise before, after, and concurrent destruction with owned workers and Temporal backstops. Generic JSON/text fuzz verdicts are direct in the callbacks; specific string, hex, status, path and endpoint targets add explicit or Go-derived admission expectations. Reader tables check the exact independently decoded path. Refused hex decoding must preserve a prepopulated destination.

The path audit replaced duplicate rows, repaired a count-overflow fixture that actually hit the trailing-separator gate, and made roots and JSON path fixtures native to macOS/Linux/Windows. Two fixture-edit failures (a compile error and an overlong temporary path against a fixed document budget) are retained in the evidence alongside their corrections.

Benchmark comparisons restore all 39 baseline test files through a Go overlay. Workload identities and input bytes match; new tests are excluded from timing builds. Each run requests 30 seconds and captures CPU, memory and its matching test binary. Actual elapsed time and iteration count are retained: Go's one-billion-iteration ceiling can end very small workloads earlier. These are single local samples, not distributions or independent acceptance receipts.


| Workload | Before ns/op / B/op / allocs | Retained ns/op / B/op / allocs |
| --- | --- | --- |
| path-component | 33.64 / 0 / 0 | 21.33 / 0 / 0 |
| relative-path | 82.22 / 0 / 0 | 109.2 / 0 / 0 |
| canonical-hex | 78.33 / 0 / 0 | 102.1 / 0 / 0 |
| digest-1k | 613.3 / 160 / 2 | 584.2 / 160 / 2 |
| digest-1m | 544690 / 160 / 2 | 476850 / 160 / 2 |
| validated-path | 1440 / 682 / 17 | 1818 / 682 / 17 |
| json-fields | 57139 / 46272 / 545 | 184930 / 46274 / 545 |
| json-fields-long | 4803996 / 7352240 / 578 | 6039521 / 7352259 / 578 |
| json-composed | 54788224 / 36544453 / 327052 | 43559794 / 36544373 / 327052 |
| json-path | 376.0 / 112 / 6 | 368.9 / 112 / 6 |
| release-compare | 971.4 / 256 / 14 | 4.869 / 0 / 0 |
| endpoint-ascii | 2860 / 144 / 1 | 2902 / 144 / 1 |
| endpoint-escaped | 5847 / 6544 / 3 | 6177 / 6544 / 3 |
| numeric-oversize | 1692393 / 1048666 / 5 | 78.76 / 72 / 3 |
| json-projection | 3037 / 1878 / 22 | 5493 / 3373 / 37 |

Release comparison's profile no longer contains formatting/parsing allocations. Oversize numeric refusal now allocates only the typed error/context wrappers; the megabyte input never reaches strconv. The numeric benchmark's MB/s is supplied input extent, **not bytes scanned**, and is deliberately omitted above.

For the measured short endpoint inputs, retained allocation counts match baseline. The exact near-limit check remains. The JSON projection benchmark now pays for Core's mandatory structural scan in addition to the fixture's semantic scan (22 to 37 allocations); that correctness cost is retained. Field and composed-document profiles are dominated by Go's jsontext namespace and decoder work. The 1 MiB digest CPU profile spends 97.88% in Go SHA-256 block processing, with the same 160 bytes/2 allocations as 1 KiB.

Unchanged control workloads show timing variability: for example the ordinary field scanner measured 57,139 → 67,504 → 184,930 ns/op across baseline/intermediate/retained stages, while allocations stayed at 545. Those timing changes do not establish a code regression or speed trend. The full attempt history is retained, and no best sample was substituted.

Core normal and race runs pass (1,928 test/subtest events in the retained race run; 85.5% statement coverage). Focused vet, staticcheck, errcheck, nilaway, witness-lint and production complexity checks pass. Linux/Windows binaries compile; native execution on those systems is unavailable here. Coverage and fuzzing are evidence, not a claim of exhaustive input-space proof or independent acceptance.

The initial diagnostic fuzz campaign preceded the intermediate after benchmarks. The final 13-target campaign runs after the retained benchmarks, serially for 30 seconds per target with one worker, explicit one-second minimization and before/after cached-corpus snapshots. All 13 final targets passed. The final audit covers 147 commands, 5,937 source snapshots and 583 artifacts with zero audit failures.

Raw profiles, matching binaries, captured output, mutations and source snapshots remain local under the ignored `testdata/test-upgrade-20260908` directory. `upgrade_evidence.json` binds their hashes and execution history. Process/Core/Contextstate batch gates and explicit user review remain before a release commit.
