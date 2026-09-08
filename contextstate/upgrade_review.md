Contextstate is ready for review. Its production path still makes one call to Go's `Context.Err`, classifies only the exact Go terminal sentinels, and returns Core-owned refusals for broken capabilities. This slice skips `recover()` on normal returns; it does not add imports, timers, goroutines, reflection, state storage, or another context implementation. Primitive's own work and auxiliary memory remain constant. The supplied `Context.Err` must itself obey Go's nonblocking contract.

The v2026.1.21 checkpoint already contained the expanded observation tables and eight-workload benchmark harness. This continuation fixes a hole in the AST observation guard, binds value-receiver method signatures to the compiler, expands the off-wire interface guard, verifies benchmark fixtures before timing, and closes the source-bound measurement report. No production correctness defect is claimed for already-correct behavior. Deliberate mutations below are test-strength evidence, not a count of production bugs.

| Public boundary | Hostile proof |
| --- | --- |
| `Validate` | Live contexts remain usable; cancellation and expiry return the exact Go sentinel. Nil interfaces, panicking methods and nonstandard errors return their owning Core refusal. |
| `Observe` | Active/cancelled/expired states remain distinct. Wrapped, joined, cyclic, noncomparable and typed-nil errors cannot impersonate a standard terminal sentinel. Custom error methods are not invoked. |
| `ObserveAfterDone` | An active `Err` is refused even when `Done` is closed. A private cancellation cause cannot relabel the terminal state. |
| `State.Validate`, `State.IsValid` | Every one of the 256 underlying values is checked against the three admitted constants. Zero and every unknown/future value are refused. |
| `State.String` | Every underlying value is checked for known versus unknown projection; all three admitted diagnostics must be distinct. Diagnostic wording is not promoted to a wire protocol. |
| `State.OffWireEnum` | Core's interface witness and 20 value/pointer probes prevent accidental JSON, streaming JSON, text, binary or append-marshaling APIs. |
| Package shape | Compiler method expressions bind signatures and value receivers. AST scans ratchet the exact public surface/imports and named observation calls. Parentheses cannot hide a direct call. These source scans are syntactic guards, not whole-program call-graph analysis. |

The observation table has 29 distinct fixtures, each crossing all three public entry points with fresh ownership. Real Go contexts cover background, TODO, values, cancellation, deadlines, private causes and detached cancellation. Hostile capabilities count `Err` calls and trap forbidden context methods. Tables perform their own typed verdicts; helpers only construct fixtures. The current package run has **637 leaf cases, 672 passing test/subtest events**, zero failures/skips and **97.7% statement coverage**. The uncovered statement is `Validate`'s defensive unknown-state fallback, which `Observe` cannot normally produce. No test-only production hook was added to manufacture that state.

There are no production structs to classify into a data-flow inventory. There is no byte/text/file/process/remote decoder or persisted wire state in this package, so there is no applicable external-ingress fuzz target. Exhausting the typed enum and attacking runtime capabilities is the relevant proof. Context cancellation/deadline transitions belong to Go; Temporal constructs deadline fixtures. The tiny exact-sentinel projection does not acquire product-policy classifier quotas or artificial wire ingress merely to pad test counts.

| Deliberate break | Observed result |
| --- | --- |
| Admit an active context through `ObserveAfterDone` | Existing hostile rows fail. |
| Relabel cancellation as expiry | Exact state/sentinel rows fail. |
| Read `Err` twice | Call-count rows fail. |
| Hide a forbidden call behind parentheses | The old source matcher fails two new leaf cases; `ast.Unparen` closes both. |
| Add standard streaming/append wire methods | Seven interface-probe leaves fail. |
| Change `IsValid` or `String` to pointer-only receivers | Compiler method-expression witnesses reject each build. |

The String mutation initially used a wrong overlay path and never started Go. Its setup failure is recorded separately and is not counted as a killed mutation. Earlier witness-lint fixture failures, the initial discarded-`recover` errcheck failure, and their corrected runs are retained too. Normal and legacy `GODEBUG=panicnil=1` observation tests pass with the retained recovery change. Exact return values and panic containment are unchanged.

The original live-context profiles attributed roughly 10–13% of sampled CPU to unconditional `runtime.gorecover`. The retained deferred function uses its existing named error result: normal return clears it, while an interrupted `Err` call retains Core's refusal and executes recovery. Go still owns the context's atomic read and cancelled-channel mechanics. Installed Go source copies are hash-bound in the local evidence; this change is not presented as a verbatim copy of `sync.OnceFunc`.

The unfiltered function summaries from all eight retained CPU profiles contain no sampled `runtime.gorecover` work. Recovery remains available for panic tests, but normal observations skip it. This profile result and the passing hostile tests support retaining the small optimization; timing samples alone would not establish a reliable speedup.

The final pairs use identical frozen test files, input contexts, benchmark identities, Go 1.27.1, Apple M1 Max, AC power and concurrency. A Go overlay restores original production for each baseline run. The harness checks `ctx.Err()` before `b.Loop` and the exact Primitive result afterward; setup is outside timing. Scoped tools/tests finish before the final serial pairs. Each run requests 30 seconds and passes CPU/memory profile paths plus a retained binary path directly to `go test`.

| Workload | Final paired baseline ns/op | Retained ns/op |
| --- | ---: | ---: |
| Validate-Live | 7.937 | 7.397 |
| Validate-Cancelled | 25.16 | 22.21 |
| Validate-DeadlineExceeded | 22.79 | 21.96 |
| Observe-Live | 7.545 | 6.721 |
| Observe-Cancelled | 33.4 | 22.23 |
| Observe-DeadlineExceeded | 31.29 | 20.86 |
| ObserveAfterDone-Cancelled | 23.21 | 22.32 |
| ObserveAfterDone-DeadlineExceeded | 22.64 | 21.81 |

All final measurements are **0 B/op and 0 allocs/op**. Whole-process memory profiles also contain Go runtime, testing and profiler setup allocations; those are distinct from timed operation allocations. All samples, including the noisier unchanged-production after pass, remain in the following history rather than selecting the best result.

Earlier sample history, ns/op (all 0 B/op / 0 allocs/op):

| Workload | Original baseline | Unchanged-production after | Preliminary recovery candidate |
| --- | ---: | ---: | ---: |
| Validate-Live | 8.202 | 10.85 | 7.06 |
| Validate-Cancelled | 23.79 | 27.65 | 21.85 |
| Validate-DeadlineExceeded | 23.02 | 37.94 | 21.44 |
| Observe-Live | 7.683 | 11.95 | 6.884 |
| Observe-Cancelled | 24.02 | 34.08 | 21.22 |
| Observe-DeadlineExceeded | 22.33 | 28.49 | 20.86 |
| ObserveAfterDone-Cancelled | 23.98 | 27.59 | 22.21 |
| ObserveAfterDone-DeadlineExceeded | 23.28 | 24.28 | 21.53 |

Each cell above is one local sample, not a stable speedup distribution. Before changing production, live Validate measured 8.202 then 10.85 ns/op with the same production bytes; that is not evidence of a code regression. The final pairs reduce time separation but do not remove scheduling noise. Go's one-billion-iteration ceiling ends these tiny operations before the requested 30 seconds in many runs. The evidence retains iteration counts, raw output, command wall duration, and effective timed duration derived from printed ns/op (with rounding disclosed). These are local engineering measurements, not independent Anvil acceptance receipts.

Retained race tests, legacy nil-panic tests, witness-lint, vet, staticcheck, errcheck, nilaway, go fix diff, goconst (minimum length 4 / occurrence 3 / no tests) and production complexity ≤10 pass. macOS executes natively; Linux/Windows amd64 test binaries compile. Native Linux/Windows execution and full-module release gates were not performed for this slice.

The final evidence audit covers **172 recorded commands, 1,706 distinct source snapshots and 696 artifacts**, with zero missing or mismatched digests and no recorded source changes during commands. One separately recorded pre-Go setup failure is additional to the command count. All 40 benchmark attempts have CPU and memory profiles and matching binaries. `upgrade_evidence.json` includes every phase, its exact command/environment/source binding, allocation and iteration counts, and the audit result.

Raw profiles, binaries, source snapshots, mutation overlays and command output stay under ignored `testdata/test-upgrade-20260908`; the committed-size report is `upgrade_evidence.json`. The user reviewed this slice and approved continuing. Its evidence manifest retains the pre-review capture status. Controlwire is next; publication belongs to the next release checkpoint.
