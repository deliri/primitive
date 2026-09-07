# Exchange upgrade review

This is an incremental review of the session, header-capture, streaming-copy,
socket-custody, replay, listener, and test-boundary changes on September
7, 2026. Exchange is **not complete or approved for release**. No commit or
repository gate was run.

## Production changes

`NewSessionClient` now requires `SessionClientRequest{PublicSuffixList: ...}`.
It admits a present caller-owned Go `cookiejar.PublicSuffixList` and constructs
an independent real `cookiejar.Jar`. Go owns matching, expiration, storage, and
synchronization. The caller owns the list’s accuracy, updates, and
concurrent-use safety. Presence validation cannot certify a domain database.
There is no nil-list fallback or embedded public-suffix database. No application
call to the old Exchange constructor was found in the four prioritized
consumers.

`headerValues` now refuses counts above `HeaderValueMaximumCount` before
allocation or value conversion. `captureHeaders` returns zero on final
validation failure. The pre-fix tests reproduced 65 accepted converted values
and invalid/partial captured fields accompanying a refusal. Public aggregate
response ingress already withheld its outward response on that refusal; this fix
also makes the inner boundary truthful and bounds conversion work.

The header-value fuzzer now uses the installed Go transport’s actual header
validation through `Transport.RegisterProtocol`, replacing its separate
`x/net/httpguts` oracle. Exchange’s production dependency closure has no
external modules. Exchange tests still transitively import `x/sys/unix` through
Filestore; Filestore’s full upgrade remains next.

`copyDownload` delegates buffer selection and `ReaderFrom` dispatch to Go's
`io.Copy`. It retains the bounded reader, cancellation checks, byte accounting,
panic containment, and excess probe. This removes an unconditional 32 KiB
scratch allocation when Go can use a smaller buffer or a destination fast path;
it does not promise every streaming workload avoids that allocation.

`replayStreamDecision` now joins cancellation with the completed attempt's
original cause. Previously cancellation discarded native read/close errors,
body-limit refusals, and typed status errors. Eight distinct failing handoff
rows reproduced this one defect. The correction is one `errors.Join` in the
existing per-operation retry decision; it introduces no scheduler, product
state, or runtime.

`ListenAddress.Validate` now matches Go TCP wildcard semantics for mapped IPv4
and zoned IPv6. Three failing representations reproduced one defect:
`[::ffff:0.0.0.0]:8080`, `[::%lo0]:8080`, and `[::ffff:0.0.0.0%lo0]:8080` were
admitted as concrete hosts even though Go treats them as wildcards. Only the
validation view strips the zone and unmaps IPv4; admitted addresses remain
exact. Go still owns parsing and listening. Subsequent runtime work deliberately
admits an explicit zero-port listen request so Go and the OS allocate the port.
An acquired listener must still report a concrete nonzero port. The earlier
mutation that admitted port zero is historical evidence for the superseded
policy, not a current refusal requirement.

`ServerListener.Address` and `ServerRuntime.Address` return the address actually
acquired from Go; dormant runtimes refuse to invent that observation. Listener
transfer checks the exact original request or the exact observed address.
Closing an untransferred listener consumes its transfer capability. The new
`ServerRuntime.Close` delegates immediate shutdown directly to `http.Server.Close`;
Go continues to own connections, request cancellation, and graceful draining.

`ServerRuntimePolicy.Validate` now rejects header extents above
`core.HTTPServerHeaderMaximumBytes` before construction. That portable ceiling
fits Go's `int` on both 32-bit and 64-bit hosts and avoids overflowing Go's
`int64` read allowance. The previous owning validator accepted limits that
construction could not represent. The retained red configuration runs reproduce
that discrepancy. A real HTTP request also succeeds at the admitted ceiling;
this does not allocate a ceiling-sized buffer or claim such a request was sent.

## Verification

- The latest full Exchange run, checkpoint 13, passed with 85.8% statement
  coverage, 2,033 passing test events and no failures or skips. This includes
  the native transport error repairs and deterministic deadline tests described
  below. Events include table parents and fuzz seeds, not distinct earned rows.
- After the user's renewed protocol correction, all 2,214 protocol lines were
  reread in five complete, untruncated ranges. The new runtime tests were
  corrected: constructor admission replaces private-conversion-only proof;
  equivalent overflow refusals are grouped; timeout projections use distinct
  neighboring fields; active-close assertions retain Go's `io.EOF`, exact handler
  count, and absent response evidence. These corrections do not establish the
  package's remaining table/matrix quotas or independent acceptance.
- Six runtime configuration mutations were killed inside fuzz callbacks. Five
  separate policy-table mutations were killed in the table tests themselves.
  Two active-request mutations were killed: replacing force close with canceled
  graceful drain, and erasing graceful shutdown's native cancellation. Every
  attempt and overlay is retained separately.
- Package checkpoint 11 passed with 85.0% statement coverage, 1,949 passing test
  events, zero failed or skipped events. Events include parents and fuzz seed
  callbacks; they are not 1,949 distinct earned rows. The runtime source and test
  evidence are uncommitted local proof, not release acceptance.

- Package checkpoint 07, after the replay and socket-wire changes: all tests
  passed, 84.0% statement coverage. This is coverage, not a confidence
  percentage.
- Six session mutations and five capture mutations were killed. Three capture
  mutations failed inside fuzz seed callbacks: exact-ceiling refusal, wrong
  valid value, and refusal of every capture.
- Three sustained Go fuzz runs, 30 seconds each with two workers: capture
  selection 311,570 executions; header value 371,681; stream custody 381,345.
  All passed. These are bounded runs, not exhaustive proofs.
- After the copy change and its profiled comparisons, stream-custody fuzz passed
  another configured 30 seconds: 490,701 executions, 30.08 seconds effective.
  Four copy mutations were killed: hidden `ReaderFrom`, bypassed byte limit,
  bypassed accounting reader, and dropped acknowledged byte count. The nine-row
  direct production-helper table also checks binary output, native failure,
  empty input, and cancellation.
- Build failures from fixture API mistakes are retained separately and are not
  counted as production regressions.
- The socket custody table now executes two distinct input documents
  independently of `wantCalls` and checks exact bounded request bytes, method,
  content type, and length. A same-length valid JSON body substitution survived
  the old table and failed the upgraded table.
- The replay handoff table now has 30 real Download-to-ReplayStream rows,
  including a genuine empty-destination neutral case and cancellation after
  read, close, status, and overflow observations. It proves the producer facts
  before the classifier runs. It is still a partial matrix, not the protocol's
  complete 50-case proof.
- The new public replay/download fuzz oracle killed five semantic mutations
  inside callbacks: lost producer cause, ignored cancellation precedence,
  substituted valid byte count, invented attempt, and refusal of every
  observation. After benchmarks it passed 63,202 executions in a configured 30
  seconds (30.18 seconds effective). Its bounded subspace covers
  payload/ceiling, OK versus service-unavailable, separate native read/close
  failures, handoff cancellation, and a one-attempt replay policy. Other status,
  schedule, combined-fault, header, and forged-callback dimensions remain open.
- The first replay fuzz seed run had an oracle mistake: it treated
  `ErrExchangeContract` as disjoint from its core-owned response/body-limit
  children. The corrected oracle preserves the error hierarchy. The failed run
  is retained and is not a production regression.
- Listener coverage replaces the padded 40-row count with 14 policy distinctions
  and 21 Go literal-grammar forwarding cases, grouping equivalent spellings
  within rows. The bounded semantic fuzzer uses Go `netip` grammar plus
  `net.IP.IsUnspecified` as the independent socket-policy oracle. Six semantic
  mutations failed inside callbacks; sustained fuzz passed 247,066 executions in
  30 configured seconds (31.06 seconds effective). This does not claim
  exhaustive address input coverage.
- Seventeen explicit `time.After` backstops in eight Exchange test files now
  call existing `Temporal.WithTimeout`, with cancellation owned by each test.
  This removes direct time execution from Exchange fixtures; it does not close
  the remaining goroutine cleanup audit. Package checkpoint 08 passed at 84.0%
  statement coverage.
- The test-only HTTP source guard follows Go AST import ownership, aliases,
  recursive carriers, and generic lexical scope. Its 33-row table passed; five
  generic-scope counterexamples failed before the matcher correction. Seven
  semantic matcher mutations were killed. An initial parameter-label mutation
  survived because it changed a branch function signatures bypass; that attempt
  remains recorded, and the corrected mutation of the actual signature path
  failed. These are source-guard repairs, not HTTP production regressions.
  Public methods and stand-alone non-struct aliases remain to be audited.

## Profiled header admission benchmark

Each leaf ran serially with `-benchtime=30s -cpu=1 -count=1 -benchmem`, CPU and
memory profiles passed in the same invocation. Both revisions used the identical
benchmark source (SHA-256
`d630afef47c839b721561486204b7090a7e1489ffb0afd16ec8ea55dce541a22`). The
benchmark drives real aggregate response ingress, checks errors, exact status
and field counts, withheld refusal output, body reads and close count each
iteration, and exact retained values after the loop.

| Input values   | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| -------------- | -----------: | ----------: | ----------: | ---------: | ---------------: | --------------: |
| 64, accepted   |        9,700 |     136,018 |      34,488 |     34,488 |               71 |              71 |
| 65, refused    |        2,936 |       1,167 |       1,760 |        144 |               71 |               5 |
| 4,096, refused |      421,399 |       345.1 |      98,448 |        144 |            4,102 |               5 |

The 4,096-value before allocation profile attributes 99.82% of allocation space
to `NewHeaderValue` and `headerValues`. Afterward, conversion is absent from the
significant allocation profile; the bounded capture carrier and error wrapping
remain. The table measures already-received response headers: it does not claim
Go’s network parser can receive arbitrarily large input for constant work.

The initial accepted-control slowdown also appeared in one explicitly justified
matched rerun: 18,389 ns/op before and 107,119 ns/op after, still 34,488 B/op
and 71 allocations on both sides. Those results remain retained. Their
historical cause has not been established.

The accepted allocation profile attributed 95.08% of allocation space to
`copyDownload`, motivating the Go copy delegation above. A new serial comparison
restored the exact original `client.go` and `stream.go` snapshots for the
reference and used the identical current benchmark for both revisions. Two pairs
reversed execution order to expose timing sensitivity:

| Execution order | Revision             |   ns/op |   B/op | allocs/op |
| --------------: | -------------------- | ------: | -----: | --------: |
|               1 | Original reference   | 198,106 | 34,488 |        71 |
|               2 | Header fix + Go copy |  33,732 |  1,721 |        71 |
|               3 | Header fix + Go copy |   7,831 |  1,721 |        71 |
|               4 | Original reference   |  17,245 | 34,488 |        71 |

Both matched pairs improved; measured allocation fell by 32,767 B/op (95.01%).
This resolves the concrete copy-allocation waste and does not reproduce a
candidate slowdown in these pairs. Large timing variation remains even for
unchanged reference bytes. A retained point observation shows competing
processes; it cannot establish historical causality. These samples do not
support a precise general speedup or whole-package performance acceptance.

The final candidate allocation profile attributes 5.17% cumulatively to
`copyDownload`, with header capture now dominant at 91.86%. The remaining
header-value allocation and grammar validation costs are recorded for further
review; their presence does not justify weakening validation or changing
ownership.

The six original commands were configured for 180 benchmark seconds total and
used about 292 command seconds. The two explicit control rechecks added 60
configured seconds and about 153 command seconds. The four repair comparisons
added 120 configured seconds and 215.33 command seconds. Calibration,
compilation, profiling, and execution are included in command time; the records
retain exact durations. Every benchmark command captured CPU and memory profiles
together.

## Evidence and remaining work

The new `BenchmarkReplayStreamDownloadHandoff` measures one real public
replay/download operation through an in-memory Go RoundTripper with a fixed
two-byte binary payload, exact output, error identities, one callback/provider
call, and one body close. It measures no real-network latency. All four attempts
used the same harness with 30-second configuration and CPU/memory profiles
together; reference runs overlay only the exact pre-fix `stream_replay.go`.

| Replay workload                       |          Before ns/op | After ns/op |       Before B/op | After B/op |  Before allocs/op | After allocs/op |
| ------------------------------------- | --------------------: | ----------: | ----------------: | ---------: | ----------------: | --------------: |
| Binary success                        |                39,939 |      99,931 |             4,184 |      4,184 |                46 |              46 |
| Cancellation plus native read failure | Semantic check failed |      30,085 | No valid baseline |      4,392 | No valid baseline |              54 |

The old cancelled path fails because it loses the native error. No performance
measurement is accepted for that failing path. The success-control timings are
single variable samples with identical allocation counts; they support no
speedup or regression conclusion. Four commands configured 120 seconds total and
consumed 146.54 command seconds; the deliberately failing reference stopped at
its correctness check. Its output and requested profiles remain retained.

Records, source snapshots, exact commands, output, profiles, mutation overlays,
and scoped artifact-hash audits are under
[testdata/test-upgrade-20260907](testdata/test-upgrade-20260907/). The audits
are `session-header-slice-artifact-audit.json` and
`copy-timing-slice-artifact-audit.json`. The latter verifies 14 run records,
including source snapshots, overlay sources, artifact hashes and lengths,
missing artifacts, and source drift. Neither is the final package manifest or
independent acceptance.

`socket-replay-slice-artifact-audit.json` additionally verifies 18 socket/replay
run records with the same checks. The supplied
[Grok review](testdata/test-upgrade-20260907/review-29d08724.md) reports no
issues in the paths it traced. It is retained as review, not independently
executed package acceptance.

`listener-temporal-architecture-slice-artifact-audit.json` verifies another 24
run records, including failed and surviving mutation attempts. It is an
author-side artifact-integrity check, not the final package manifest or
independent acceptance.

## Profiled listener admission

`BenchmarkListenAddressAdmission` measures public literal parsing, validation,
and exact canonical projection. It performs no DNS resolution or listener
effect. Five leaves use the same harness before and after, with only the exact
pre-fix `server_runtime.go` overlaid for the reference. Each of the ten
comparative invocations specifies 30 seconds, one CPU, and CPU/memory profiles
together. Package checkpoint 09 passed before the comparison, at 84.0% statement
coverage.

| Workload              |          Before ns/op | After ns/op |       Before B/op | After B/op |  Before allocs/op | After allocs/op |
| --------------------- | --------------------: | ----------: | ----------------: | ---------: | ----------------: | --------------: |
| Concrete IPv4         |                 85.42 |       97.56 |                16 |         16 |                 1 |               1 |
| Concrete mapped IPv4  |                 133.6 |       108.7 |                24 |         24 |                 1 |               1 |
| Mapped wildcard       | Semantic check failed |       81.13 | No valid baseline |          0 | No valid baseline |               0 |
| Zoned wildcard        | Semantic check failed |       83.29 | No valid baseline |          0 | No valid baseline |               0 |
| Zoned mapped wildcard | Semantic check failed |       111.9 | No valid baseline |          0 | No valid baseline |               0 |

The old revision incorrectly admits all three wildcard forms and therefore fails
those benchmark correctness checks. Their failed outputs and requested profiles
remain retained. Accepted controls retain their allocation counts; their timings
moved in opposite directions. Single samples establish neither a performance
regression nor an optimization. Zero-allocation refusal measurements describe
bounded, already-supplied literals, not network processing.

The accepted candidate allocation profiles attribute 99.94% (IPv4) and 99.95%
(mapped IPv4) of sampled allocation space to Go `netip.AddrPort.String`. The CPU
profiles show Go parsing/formatting, Exchange validation, assertion cost, and
runtime activity. The wildcard allocation profiles primarily contain
test/profiling startup costs, consistent with the measured zero per-operation
allocations. These workloads repeat fixed literals; the zone has already been
interned by Go during calibration, so they do not measure first-use or
continuously changing zone allocation. No Primitive cache or alternative parser
was added.

The comparison configured 300 benchmark seconds and consumed 294.86 command
seconds, including the three expected early correctness failures. Afterward,
listener fuzz passed 506,031 executions in a configured 30 seconds (30.54
seconds reported package execution).
`listener-profiles-slice-artifact-audit.json` verifies 27 records covering the
comparison, profile analysis, checkpoint, follow-up fuzz, and excluded harness
attempt. Every requested comparison profile is present, with matching retained
hashes and lengths and no source drift.

An initial one-iteration benchmark harness check omitted profiles. That
procedural miss remains in `listen-address-benchmark-semantic-check.json` and is
excluded from performance evidence. All ten comparative runs capture both
profiles with the required duration configuration.

The earlier sweep recorded broad package-wide audit notes about table quality,
external-door inventory, source guards, and benchmark coverage. Those notes
are not a list of additional reproduced production defects. The four concrete
follow-up issues raised subsequently are resolved below. This record does not
claim independent package-wide protocol or release acceptance. Filestore has
not begun its full upgrade.

## Completed transport errors and deterministic deadlines

Aggregate attempt classification now joins operation cancellation with the
completed producer error. Its previous implementation erased native read/close
errors and body-limit refusal. The focused producer-to-classifier table checks
the producer facts before classification. The public bounded-client fuzzer
checks both `SendBounded` and `SendNoBodyBounded`, including exact retained body,
status, attempt, read and close counts. Five mutations failed inside its
callbacks: cause erasure, valid byte substitution, invented attempt count, and
refusal of every call at either public door. This focused proof does not replace
the complete producer/classifier matrix still owed by the package.

The same cause-erasure defect existed earlier when Go's `http.Client.Do`
returned an error concurrently with context termination. Aggregate execution
and streaming transport classification now retain Go's error with
`errors.Join`. The public-door table exercises both bounded clients, Upload,
Download and RoundTripStream. It checks native error identity, the original
`net.OpError`, Go's `url.Error`, and absence of response or transfer evidence
when no response was obtained. Three separately overlaid mutations fail:
aggregate native-cause erasure, stream operation-cancellation erasure, and
stream attempt-deadline erasure. The latter is exercised while the operation
context remains live, so the operation branch cannot mask it.

One- and two-nanosecond attempt deadlines execute in Go's `testing/synctest`
bubbles, with time observations and duration construction through Temporal.
Go owns clock advancement and goroutine joining. The real HTTP context-budget
table separately checks exact bounded output, overflow withholding, empty-body
neutrality, HEAD metadata, and refusal before any HTTP effect. This replaces
the old real-network one-second timing race and its mislabeled neutral refusal.
The combined focused run passed all 57 events.

## Profiled runtime and aggregate handoff

Every comparative leaf specifies 30 seconds, one CPU, and both CPU and memory
profiles in the same invocation. The runtime benchmark measures dormant
construction or acquire/close of an OS-allocated listener; it does not establish
connections or measure HTTP throughput.

| Runtime workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Dormant construction | 1,094 | 307.4 | 544 | 560 | 4 | 4 |
| OS-allocated listener, initial repair | Semantic check failed | 11,569 | No valid baseline | 608 | No valid baseline | 18 |
| Same listener, typed Go API refinement | 11,569 | 9,081 | 608 | 544 | 18 | 16 |

The old listener policy rejected port zero and fails the correctness check, so
there is no accepted numeric reference for that behavior. Profiles of the
initial repair exposed resolver work despite an already parsed numeric address.
`Listen` now calls Go's `net.ListenTCP` with `net.TCPAddrFromAddrPort`; Go still
owns socket creation, defaults, and the OS boundary. The matched refinement
removes 64 B/op and two allocations. The final CPU profile is dominated by
system calls. Dormant construction's extra 16 B/op remains recorded. These
single timing samples do not support a general speedup claim.

The aggregate handoff benchmark measures a real aggregate producer and
classifier through an in-memory Go RoundTripper, with a fixed two-byte binary
response. It checks exact output, errors and effects on each iteration.

| Aggregate workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Completed binary response | 4,090 | 12,600 | 2,678 | 2,678 | 32 | 32 |
| Cancellation after native close failure | Semantic check failed | 10,092 | No valid baseline | 2,886 | No valid baseline | 40 |

The canceled reference loses the native cause and has no accepted performance
baseline. The control's slower timing remains unresolved as a performance
comparison; unchanged allocation does not prove unchanged latency. CPU sampling
captured 29.45 CPU seconds over 47.32 seconds before and 11.15 over 48.40 seconds
after, with substantial Go runtime activity. Those observations do not establish
why the timing changed. Allocation profiles are dominated by Go request/header
construction and context ownership; the canceled path additionally retains
joined errors. No alternate timer, allocator, or transport was introduced.
These profiles predate the later native-transport error repairs, whose branches
this successful-response workload does not execute. Exact source snapshots bind
each result. All four attempts, including the deliberately failing reference,
and six CPU/memory analyses are retained.

After runtime benchmarking, four serial 30-second fuzz runs passed: bearer
token admission (317,400 executions), bearer receiving (463,666), runtime
configuration (282,685), and current listener policy (263,469). Each used two
Go workers. These bounded runs supplement the semantic mutation controls;
they do not establish exhaustive ingress coverage.

The public bounded-client producer-cause fuzzer subsequently passed 446,505
executions in 30 configured seconds (30.02 seconds inside the callback run),
with two Go workers. `runtime-auth-aggregate-native-slice-artifact-audit.json`
checks 82 additional run records and 654 distinct retained files, including
failed attempts. All recorded artifact hashes and lengths matched; no requested
artifacts were missing and no source drift was recorded. This is author-side
artifact verification, not package completeness or independent acceptance.

## Four concrete follow-up issues

The two address fuzzers now decide independently whether each input must be
accepted, then check the exact prefix set or client address. Correctly typed
refusal no longer makes valid input pass. Prefix seeds come from a validated
nominal carrier and its real projection, so a broken parser cannot prevent the
callback from running. Address seeds likewise use validated address projections.
Five initial semantic mutations were caught. A sixth, trusting a leading
forwarded address, survived a seed with only two members; a three-member seed
now catches it. Both attempts remain recorded. The sustained runs passed
312,042 prefix and 307,196 client-address executions, 30 configured seconds
each with two workers.

The real Go Expect/continue test reproduced a misleading upload observation:
zero source reads accompanied a response claiming two transferred bytes.
`Upload` now reports zero in `Metadata.Bytes` and retains the request declaration
separately as `DeclaredRequestBytes`. `StreamRoundTripResponse.RequestBytes`
is renamed to `DeclaredRequestBytes`; its response byte count still measures
actual destination acknowledgements. This is an API/behavior change for review:
a declared request extent is not a delivery receipt. Go owns the transport,
source dispatch, and file fast paths; no counting reader or transport runtime
was introduced. Go's own request writer can return nil after declining to send
an expected body, so even `WroteRequest(nil)` would not prove delivery.

Both public upload doors have real early-response tests and a semantic fuzzer
covering no, partial, and complete source consumption, response/status failures,
exact declarations, destination bytes, and caller-owned source closure. Six
mutations were caught inside callbacks. The 30-second sustained run passed
176,586 executions. A separate replay test kills a mutation that treats a
nonzero declaration without its HTTP metadata as an absent observation.
The affected Objectstore and PayPal packages compile; the only required
consumer edit was PayPal's test reference to the renamed field. Objectstore
already uses its own observed source count for upload integrity. No references
to the removed field were found in the four prioritized applications' own Go
source; their vendored versions were not edited.

The [handoff matrix](handoff_matrix_review.md) now exhausts the finite status
and error-identity decision domains, with exclusive primary classes and real
producer facts checked before classification. Every admitted status is tested
against both an exact and a different caller expectation, including expected
5xx responses. All 128 identity subsets are tested with reversal, duplication,
and cancellation. A compiler-bound source check detects new classifier Core
identities missing from that fixture. This found one production defect:
aggregate classification retried a typed request refusal. It now stops and
retains the original cause, as streaming already did. Nine decision/retention
mutations are checked against the completed matrix. Generated executions are
not claimed as thousands of distinct earned regression rows.

The final full Exchange run, checkpoint 17, passed at 86.0% statement coverage,
with 7,210 passing test events and no failures or skips. Coverage is not a
confidence percentage. Repository gates and commits remain deferred.

## Timing investigation and final profiles

The original aggregate success control measured 4,090 ns/op before and 12,600
after. Four additional serial runs restored the exact original before/after
`client.go` snapshots, kept the benchmark unchanged, and reversed run order.
Every invocation specified 30 seconds and captured both profiles.

| Order | Revision | ns/op | B/op | allocs/op |
| ---: | --- | ---: | ---: | ---: |
| 1 | Original before | 4,697 | 2,678 | 32 |
| 2 | Original after | 3,319 | 2,678 | 32 |
| 3 | Original after | 2,655 | 2,678 | 32 |
| 4 | Original before | 2,883 | 2,678 | 32 |

Both paired comparisons put the candidate below its reference. The original
threefold slowdown did not reproduce. Identical after bytes varied from 12,600
to 2,655 ns/op across retained runs, so that single disparity cannot establish
a code regression. Memory profiles retain the same dominant Go request/header
and context allocations. CPU samples show substantial runtime activity and
different sampled-CPU/elapsed proportions. The historical cause of the original
sample remains unknown; these measurements justify neither a precise speedup
nor a package-wide latency claim.

The final production candidate also passed three separately profiled,
30-second workloads:

| Workload | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Aggregate binary response | 7,707 | 2,678 | 32 |
| Aggregate cancellation after native close failure | 10,921 | 2,886 | 40 |
| 10 MiB file upload over loopback | 8,118,610 | 42,272 | 132 |

The slower final aggregate sample remains included. Its allocation profile
again attributes the dominant costs to Go request/header and context ownership.
The upload measured 1,291.57 MB/s on this machine's loopback path, not WAN
throughput. Its CPU profile attributes 97.72% to system calls; Go `io.copyBuffer`
accounts for 74.83% of sampled allocation space. It uses bounded working memory
for a 10 MiB stream and verifies the server's actual received extent separately
from the client's declaration. No optimization bypasses Go's file or network
execution.

The timing recheck configured 120 benchmark seconds; the three final workloads
configured another 90. Their exact durations, sample counts, CPU/memory
profiles, binaries, source snapshots, and all 14 profile analyses are retained.
The three sustained fuzz runs followed benchmarking. The final status-table
extension and its validation came afterward; production and benchmark sources
did not change during or after these final profile/fuzz runs.

`four-concrete-followup-artifact-audit.json` records the scoped integrity check
for this follow-up, including failed attempts, surviving mutations, final
mutation rechecks, retained source snapshots, profiles, and review documents.
It is author-side verification of these artifacts, not independent release
acceptance or a replacement for the deferred repository gate.
