# Exchange continuation after checkpoint 40d7c8e

The author-side Exchange continuation is ready for review. The final package
run passed, all 35 current fuzz targets have sustained receipts, and all 38
benchmark workloads have successful same-invocation CPU/memory profiles. The
settled-source reconciliation at the end supersedes the historical work lists
below. Repository gates, race runs, linters and release work remain deferred by
the user. Current changes have not been committed or independently accepted.
The authorized Filestore sweep follows this evidence closure.

## Production changes

- Basic authorization registers deferred clearing of its entire fixed decoding
  array before decoding can partially write and fail. Delimiter splitting uses
  Go's `bytes.Cut`.
- Basic identity validation operates directly on its nominal string. Secret
  validation operates directly on bytes. Parsing constructs the existing typed
  request with one owned identity and one cloned secret; it no longer allocates
  intermediate credential strings or repeatedly copies credentials to validate
  them. The public API is unchanged.
- `IdempotencyKey.Validate` owns its existing length/alphabet rule. Parsing
  constructs the nominal value and calls its validation. This also allows parser
  refusal mutations to reach fuzz callbacks without breaking seed construction.

The execution remains Go's Base64, Unicode, byte/string and HTTP machinery.
Certificate verification tests use Go's x509 and TLS implementation; Exchange's
certificate observer remains a digest projection of Go's verified chain.

## Retained measurements

Each measurement below used one serial 30-second benchmark leaf, CPU=1, with CPU
and memory profiles from that same invocation. Exact commands, source snapshots,
machine/toolchain information, counts and artifact hashes are recorded in
`testdata/test-upgrade-20260907/post-checkpoint-*.json`. Raw artifacts remain local.

| Workload | Original ns/op, B/op, allocs/op | Cleanup/validation intermediate | Final credential ownership |
| --- | --- | --- | --- |
| Maximum idempotency key | 135.1, 0, 0 | 154.3, 0, 0 | unchanged from intermediate |
| Maximum Basic credentials | 5373, 3856, 7 | 6285, 3856, 7 | 6084, 1296, 3 |
| Partial Base64 refusal | 3237, 80, 4 | 3250, 80, 4 | 3227, 80, 4 |

The maximum accepted credential operation removes 2,560 allocated bytes and
four allocations. Profiles identified credential validation copies and now
attribute the remaining allocation predominantly to the owned secret and
identity. These samples do **not** establish a latency improvement. The key and
accepted-credential elapsed timings are higher than their original samples;
every attempt is retained rather than selecting a favorable one.

## Verification so far

The latest full package run before the constructor and socket-observation fuzz
additions is `post-checkpoint-typed-auth-package-checkpoint-19`: 8,348 passing Go
test events, no failures/skips, 86.4% statement coverage. These event counts
include parents and fuzz seeds; they are not earned hostile-table row counts.

New semantic fuzz coverage includes Method JSON, Basic identity parsing, Basic
header construction and receive custody, idempotency keys, socket routes, all
five JSON receive doors, bounded/no-body receive, socket header/query/path
observations and Go-verified certificate identity. Constructor and the newest
socket targets still need their sustained runs and mutation controls.

Retained 30-second configured fuzz attempts completed with 438 Method, 514,385
Basic identity, 480,475 idempotency key, 72,808 route, 4,156 Basic receive,
346,929 JSON receive and 452,306 bounded/no-body receive executions. Large-seed
minimization stalled some targets; those low execution counts are not presented
as broad exploration.

Eight nominal-parser mutations were killed inside fuzz callbacks. Sixteen of
seventeen receive mutations were killed. Removing just the projection's final
validation survived because the resulting `Received.Validate` independently
enforces the same completed-value contract. Removing both completed-value
checks was killed by the no-op projection table row. That survivor is retained
and explained, not relabeled a kill.

The first JSON receive fuzz attempt exposed a mistaken test oracle: the
structure-only projection decoder permits JSON null before projection. The
oracle was corrected to use Go's value decoding for that door. This was a test
harness error, not a production regression. The original failed attempt remains
in the evidence bundle.

## Remaining acceptance work

- Complete the compiler-bound public-ingress/fuzz inventory and its drift guard.
- Cover remaining public client JSON/provider response doors, SDK configuration
  inputs, and remaining material-admitting output/session boundaries according
  to their actual ownership contracts.
- Finish the legacy table audit, including local triads, earned boundary rows,
  goroutine ownership and finite-domain justifications. Fuzzing and coverage
  percentages do not waive these requirements.
- Finish sustained fuzz and callback mutation controls for new targets; retain
  survivors and failed harness attempts accurately.
- Complete final benchmark coverage and exact-source package verification, then
  rebuild the report/artifact manifest. Do not include large raw artifacts in a
  future push.

## Subsequent ingress completion checkpoint

`post-checkpoint-ingress-package-checkpoint-21` passed with 8,893 Go test events,
no failures/skips and 88.2% statement coverage. Subsequent source edits require
another final run. Method token production now references `net/http` constants;
the finite nominal-domain test uses direct table assertions for all 256 values.

The executable inventory resolves 94 public functions/methods to actual Go
function values and fuzz/capability/projection classifications. Its first
behavioral run exposed 15 deliberately unfinished entries; all now have coverage
bindings. Its own structural drift controls and classification audit remain work
in progress. The successful inventory is wiring evidence, not blanket behavioral
acceptance.

New coverage includes every public JSON client door, all SDK configuration
constructors, session-cookie custody, all typed/standard response writers and
streamed round-trip responses. `FuzzReplayStreamPreservesDownloadObservation`
was renamed to `FuzzReplayStreamPreservesDownloadAndRoundTripObservation` and now
executes both producers before replay classification. Existing historical
receipts retain their original target identity and source snapshots.

The newest completed sustained runs retained these execution counts:

| Target | Executions |
| --- | ---: |
| Basic header construction | 251,503 |
| Socket header/query/path | 450,376 |
| Go-verified certificate identity | 390,886 |
| Five JSON client response doors | 243,906 |
| SDK configuration | 27,628 |
| Corrected session-cookie oracle | 362,706 |
| Standard response writers | 1,996 |
| Typed material writers | 428,079 |
| Download/round-trip replay observations | 373,749 |

SDK and standard-writer fuzzing spent much of their configured duration in
minimization. Those runs passed but their execution counts are retained as a
limitation, not disguised as broad exploration.

Cookie fuzzing first failed because its oracle compared jar-storage `Quoted`
metadata with request-wire metadata. Go correctly quotes a value containing a
space during `Request.AddCookie`. The independent oracle now performs that Go
serialization before comparison. The minimized `0= 0` input remains under
`testdata/fuzz/FuzzSessionClientGoCookieCustody/bcbda9607e8257cd`. This was a test
oracle error, not a production defect. The evidence harness now snapshots corpus
inputs/outputs and its own source as well as package source.

Thirteen socket/Basic-constructor mutations and six JSON-client mutations were
killed inside callbacks. Of the next 31 mutation attempts, 28 were callback
kills and three failed compilation because the mutation script referenced an
absent Core status constructor. All three were corrected to use
`HTTPStatusCode.AdmitInt` and killed inside callbacks. Both attempts are retained;
compile failures are not reported as semantic kills.

`BenchmarkSocketVerifiedClientCertificateDigest` used a 312-byte Go-verified
certificate: 171,656,528 iterations, 209.7 ns/op, 0 B/op, 0 allocs/op. It ran with
the same serial 30-second/profile configuration as other retained benchmarks.
CPU samples predominantly execute Go's SHA-256 implementation; allocation
samples belong to runtime/profiling setup. This is a baseline measurement, not
an optimization comparison.

No claim of 100% confidence, complete protocol conformance, final performance
acceptance or independent review is made by this checkpoint document.

## September 7 continuation through package checkpoint 24

The full package checkpoint `post-checkpoint-exchange-package-checkpoint-24`
passed 9,081 Go test events, no failures/skips, at 88.3% statement coverage.
Subsequent buffer-panic and shared-header edits require a later full run.

The small positive declared-body optimization uses Go `io.Copy` and
`io.LimitReader` for a declaration-plus-one prefix, then the existing bounded
copy for any continuation. It does not treat a declaration as proof of EOF or
widen the caller's ceiling. Exact one-byte overflow, native read/close causes,
understatement continuation, cancellation, and initial read offers have named
hostile tables and a public `ReceiveBounded` fuzzer.

| JSON workload | Before ns/op; B/op; allocations | Candidate ns/op; B/op; allocations |
| --- | --- | --- |
| 128 B | 12,317; 45,512; 97 | 8,468; 12,866; 96 |
| 1 KiB | 19,009; 53,834; 105 | 15,429; 22,196; 104 |
| 8 KiB control | 74,910; 148,317; 113 | 74,151; 148,317; 113 |

The reverse-order 128-byte pair was candidate 8,480 and reference 12,325 ns/op,
with identical allocations to their first pair. The allocation profile before
attributed 71.92% of allocation volume to `io.copyBuffer`; the candidate removes
its oversized initial scratch allocation. The candidate CPU profile was
91.29% `runtime.kevent`, so it does not support useful application CPU
attribution. All profiles and attempts remain retained; no package-wide latency
claim follows from these narrow measurements.

The declared-extent fuzzer completed 425,583 executions in its configured
30 seconds. Five damaging prefix controls failed inside callbacks. Moving the
split from declaration-plus-one to declaration survived: the continuation still
returned exact bytes, so this is retained as an equivalent control, not a kill.

Additional review work:

- SDK active reads now cancel and join the owned client worker on every exit;
  completed responses survive late cancellation. Closed `net.Pipe` transport
  fixtures replace the released-port race.
- Ingress projection exemptions require exact Go signatures. Foreign runtime
  package functions cannot impersonate bindings. Missing/duplicate bindings,
  nil targets and nil capability proofs are rejected. An overlay-only added
  source declaration was invisible to the on-disk AST scan; the corrected
  temporary on-disk mutation was rejected and the file restored. Both attempts
  are retained.
- Raw HTTP architecture scanning includes methods, aliases, defined carriers,
  interfaces and mutable exported variables. Its two compiler-bound permitted
  doors are socket admission and the SDK adapter's required Go RoundTripper
  method. The first stricter scan incorrectly omitted that required method;
  its failed attempt and corrected proof remain in the bundle.
- The Basic storage guard rejects shadowed `clear` bindings. Runtime credential
  custody remains separately tested; this is deliberately a narrow source
  invariant, not a general data-flow analyzer.
- Proxy derivation, standard writers, inventory JSON writing and route grammar
  use direct hostile tables. Basic identity JSON includes C0/C1 boundaries and
  truncated/overlong UTF-8. Header fuzz canonical seeds come from validated
  nominal projections; refuse-all and wrong-valid constructors both fail
  inside callbacks.
- Streaming benchmarks now have bounded Temporal waits for handler observations.
  The large declared-body benchmark no longer falsely claims a full reservation.
  New address benchmarks check exact direct/trusted/untrusted/cloud projections.

The strengthened response-buffer fuzzer first completed 366,639 executions and
killed six semantic controls. The later read-through found a separate production
hole: destination Header/WriteHeader/Write panics escaped containment. The red
fixture had three failing hostile rows (plus their parent), while normal and
empty controls passed. The fix reuses `containResponseWriterPanic` with named
results, retaining completed acknowledgments. Further rows distinguish an effect
performed before panic from a completed acknowledgment. A zero receipt is absent
evidence and must not be described as proof that a faulty writer made no effect.
Both new panic mutations fail inside fuzz callbacks. A new fuzz assertion
initially misread Core's parent error hierarchy; its corrected receipt/cause
proof passed, and the failed attempt is retained as a harness error.

The complete benchmark sequence paused cleanly after leaves 01–05 for that
production repair. Those five successful profiled runs are intermediate source
measurements, not a completed final sequence. A new ResponseBuffer baseline
before the panic fix measured 428.8 ns/op, 672 B/op, 6 allocations for 128 bytes
with same-invocation CPU/memory profiles. Its post-fix comparison is still owed.

Core now owns a typed Trailer field accessor used by BufferResponse, which also
uses the existing Content-Length accessor. The focused header accessor test
exposed the earlier omission of Expect from its inventory; the corrected
compiler-bound table includes all eleven accessors. This is supporting contract
work, not acceptance of the Core package sweep.

Still owed: finish the remaining manual table/goroutine review, finish the
profiled benchmark sequence on the settled source, rerun the changed fuzzers
(including shorter native minimization budgets for previously stalled targets),
perform final package verification, and reconcile source/report/artifact/corpus
manifests. No repository gates, race runs, release actions, new commits, or push
have been performed in this continuation. Filestore has not yet been started.

## Settled source and evidence reconciliation

The remaining author-side Exchange work listed in the historical checkpoints
above is complete. `post-checkpoint-exchange-final-source-tests-28` passed
9,384 Go events, with zero failures/skips and 88.2% statement coverage. This is
the final source after repairing the loopback benchmark fixture. Production is
identical across the 38 accepted benchmark leaves; leaves 01–37 retain the
earlier benchmark-file snapshot. Counts include parent
events, exhaustive finite-domain cases and fuzz seeds; they are not earned-row
counts. Supporting Core header proof passed twelve events; this does not claim
completion of the Core package sweep.

The manual read-through covers all 115 current Exchange test files and the
production files. The final corrections replaced the observed-response,
zero-extent and complete address-authority domain checks with direct named
tables, removed the remaining error-substring assertions in retry observations,
and used Go cookie parsing for response-cookie observations. The completed
response table clones nested header values before mutations so a mutation cannot
silently alter its oracle. Four final damaging response/extent controls failed
inside their intended tables.

Two final sustained fuzz failures were harness defects, not production defects:

- Go's recorder acknowledges no body bytes for a 304 response and returns
  `http.ErrBodyNotAllowed`, while `http.Redirect` ignores that write error.
  Exchange correctly retains the native error. The corrected independent Go
  writer oracle retains the actual write cause; named GET/HEAD/POST boundary
  rows and the minimized input remain. The corrected run executed 322,065
  inputs. Suppressing the retained standard-writer error failed inside the fuzz
  callback.
- Creating an SDK loopback server per input exhausted native TCP ports after
  11,281 executions. That fuzzer now tests its response representation through
  the real SDK adapter with a Go RoundTripper seam and counted native reads and
  closes. Real HTTP framing remains covered by the separate transport fuzzer.
  Its corrected run executed 408,601 inputs. Disabling streaming selection
  failed inside the callback. The minimized resource-failure input and both
  failed attempts remain, including the intermediate unused-import build error.

The latest successful sustained receipt for every one of the 35 current fuzz
functions is enumerated in
`post-checkpoint-final-fuzz-reconciliation.json`. Each receipt keeps its actual
source snapshot. Earlier unchanged targets are historical runs, not relabeled
final-source runs. The final full package run executes the complete current seed
corpus. Failed builds, oracle corrections, low-exploration attempts, equivalent
survivors and corrected callback kills remain distinguishable in the evidence.

The original final loopback benchmark exhausted native TCP ports with Go's
default two-idle-connection pool under eight workers. Its fixture now sets Go's
active and idle connection limits to the worker count and observes accepted
connections through `http.Server.ConnState`. A disabled-keep-alive mutation
violates the exact connection-count oracle. This is a corrected workload
definition, not a claimed production performance gain. The failed attempt is
retained.

All 38 accepted benchmark leaves have 30-second same-invocation CPU/memory profiles and
retained binaries. [The final benchmark report](final_benchmarks.md) records every
measurement and the supported before/after comparisons. The 128-byte buffer
release comparison is 428.8 before versus 428.4 ns/op after panic containment,
with the same 672 B/op and six allocations. It provides no evidence of a material
regression. Basic ownership reduces allocations; its elapsed time has no claimed
improvement. Runtime/poller-dominated CPU samples are disclosed rather than used
to invent application-level CPU attribution.

`post-checkpoint-final-artifact-audit.json` verifies the continuation command
records against retained source, corpus, overlay, profile, binary and log bytes.
It also inventories the final reports and copies the evidence scripts and the
Go source consulted. This is author-side integrity checking, not independent
acceptance. Raw evidence remains local; hashes in a report do not imply remote
backup of those bytes.

The work is ready for user review, and the authorized Filestore sweep may now
begin. Repository gates, race runs and linters remain deliberately deferred;
there is no claim of release acceptance, all-platform execution or absolute
confidence. No new commit or push occurs without explicit user approval.
