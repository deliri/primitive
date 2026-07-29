# Probe package interview

Status: `COMPLETE` | Decision: `RETIRE`

This is the sole reconstruction report for archived package `probe`.
The archive is evidence, not authority. No archived source was copied.

The archived package tries to provide one generic foreground conformance
runner for two unrelated capabilities: network exchange and durable local
filestore. That is not yet an admitted Primitive responsibility.

The need for foreground diagnostics is real, but the four consumers show that
the evidence must remain bound to the operation being proved:

- Kernel readiness invokes the real store capability and owns health/readiness
  state;
- Witness and Bug execute the exact candidate binary and bind its typed
  self-test result to the signed release target;
- Peachfuzz executes the governed Go toolchain and retains a typed, bounded
  process failure; and
- archived Exchange and Filestore already own the facts required to prove
  their operations.

The archived Probe inserts an adapter between the operation and the report.
That adapter authors the evidence Probe later calls proof. The native
filestore adapter unconditionally asserts stream completion and file sync and
projects one Filestore activation value into both activation and directory-sync
booleans. The Exchange adapter supplies its own correlation and byte counts.
Probe can validate the shape of those assertions, but it cannot independently
establish their truth.

The package also has verified contract defects: exchange byte counts are never
validated; a valid success test deliberately uses `math.MaxUint64` for both
counts; failed Exchange reports may retain unvalidated evidence; report
validation does not close unused fixed storage or enforce the step state
machine; cleanup failure erases the primary failure identity; late clock
failure can erase a report after an external effect; several specification-
required Filestore hostile paths are absent; the public API is implemented as
one-line wrappers; and the payload algorithm contains raw magic values.

The admission verdict is therefore:

1. do not recreate a production `probe` package in Primitive 2026;
2. do not copy its five Probe error identities or its package-private wire
   tokens into Core;
3. preserve the bounded O(1) payload/verifier mechanics, fixed evidence
   storage, panic containment, error-disclosure discipline, and native-path
   test style as patterns at the actual capability owner;
4. keep operational diagnostics in the consumer or protocol package that can
   bind the report to the real target and action; and
5. reconsider a shared package only after at least two production consumers
   need the same product-neutral typed operation and can prove it without
   self-attesting adapters.

## Evidence boundary


### Source revisions and Primitive pins

| Source | Exact revision or Primitive pin | Archived `probe` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`); Probe tree `99cde2c62ee9e565c00a2189b25fd961152c364d` | Present. Introduced by `cf8fa882ee56bb263220a1466c6deae08e466018` on `2026-07-24T15:52`, `2026-07-24T15:26-04`, `2026-07-24T15:00`; modified by `d259789e87bcadb829c5ffac72c6c91ccc604098` on `2026-07-25T21:01`, `2026-07-25T21:06-04`, `2026-07-25T21:00`. | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; Probe and its inspected Core contracts are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` (`2026-07-23T04:01`, `2026-07-23T04:52-04`, `2026-07-23T04:00`) | Committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20`, which predates Probe. Dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43`, whose tree contains Probe; Kernel has no production import at either frontier. | Broad pre-existing dirty migration. The committed and dirty pins are distinct evidence. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` (`2026-07-24T15:52`, `2026-07-24T15:58-04`, `2026-07-24T15:00`) | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`, which predates Probe. No production file imports Probe. | Only the pre-existing untracked ledger was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` (`2026-07-24T15:52`, `2026-07-24T15:54-04`, `2026-07-24T15:00`) | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3`, which predates Probe. No production file imports Probe. | Only the pre-existing untracked ledger was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` (`2026-07-24T15:52`, `2026-07-24T15:50-04`, `2026-07-24T15:00`) | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`, which predates Probe. No production file imports Probe. | Only the pre-existing modified ledger was observed. |

Every committed consumer pin predates Probe's introduction. Exact committed
searches found no import of `foundation/v2026/probe` in Kernel, Witness, Bug,
or Peachfuzz. The dirty Kernel Primitive pin contains the archive-era package,
but Kernel has no Probe import at that frontier either.

Witness, Bug, and Peachfuzz each contain a temporary
`protocol/_foundation_source/core/error_identity.go` snapshot with copied
`ErrProbeContract`, `ErrProbeCapability`, `ErrProbeCorrelation`,
`ErrProbeIntegrity`, and `ErrProbeCleanup` declarations at lines `33-37`.
Those orphan copied identities are not a Probe use or cutover. They are exactly
the hidden-coupling shape Primitive 2026 must not perpetuate.

At archive HEAD, Probe contains:

- 1,291 production Go lines;
- 1,312 Go test lines;
- a 175-line specification;
- 29 Probe-specific Core constant lines;
- sixteen named deterministic tests;
- no fuzz target; and
- no benchmark.

The archive ledger marks the package complete and claims native, race/shuffle,
cross-build, vet, static, security, lint, complexity, and witness-lint proof
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1016-1041`). A fresh focused run during this
interview passed:

```text
go test ./probe
```

That establishes internal deterministic coherence on the implemented contract.
It does not prove that the contract is truthful, complete, or demanded by a
consumer.

## Capability ownership

The stated purpose is to produce bounded foreground evidence that an injected
Exchange or Filestore capability satisfies a narrow operational contract. It
describes itself as a socket tester rather than a second implementation,
forbids direct imports of those sibling packages, and requires composition
roots to adapt the real implementation to Probe's interfaces
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:6-25`).

Its public operation surface is:

```go
func CheckExchange[T ExchangeTarget](
    context.Context,
    ExchangeRequest[T],
) (ExchangeReport, error)

func CheckFileStore[T FileStoreTarget](
    context.Context,
    FileStoreRequest[T],
) (FileStoreReport, error)
```

The package intends to own:

- a generic typed target-validation boundary;
- one injected clock and monotonic elapsed observations;
- synchronous panic containment around clocks and capabilities;
- an Exchange check over correlation, attempts, status, and byte counts;
- a Filestore lifecycle of inspect, commit, read, verify, remove, and final
  inspect;
- a deterministic one-byte-to-one-MiB streaming artifact;
- exact extent and SHA-256 verification;
- distinct immutable Exchange and Filestore reports;
- a closed step/outcome/failure vocabulary;
- fixed-capacity report state;
- projection of hostile external errors onto stable Core identities; and
- O(1) retained memory in artifact size
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:27-169`).

It deliberately excludes scheduling, monitoring history, endpoint discovery,
credential inference, broad disk testing, benchmarking, HTTP handlers, and
product readiness claims (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:171-175`).

This looks narrow on paper, but the two operations have no common semantic
center beyond invoking an interface and reporting what it said. Exchange has one
operation and a correlation fact. Filestore has a destructive six-step
lifecycle and durability claims. A generic package cannot independently prove
either while being forbidden to invoke the concrete primitives.

## Archive evidence

### Archived strengths worth preserving

### 1. Bounded streaming integrity mechanics

The Filestore path generates its payload by index and streams it through a
fixed 4,096-byte buffer. It hashes expected bytes without retaining the
payload and verifies exact length plus SHA-256 through an `io.Writer`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/payload.go:13-129`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/probe_constants.go:19-20`).

The verifier rejects:

- short reads;
- extra bytes;
- content mutation;
- writer-count impossibility; and
- digest mismatch.

This is the right O(1)-memory shape. The implementation should not be copied
as a package, but the technique is useful in the real Filestore production-path
tests and in any consumer-owned destructive storage diagnostic.

### 2. Synchronous panic containment without fake timeout goroutines

Clock, target, and capability calls are panic-contained. Probe does not launch
a goroutine and pretend cancellation proves an uncooperative capability
stopped. The specification explicitly leaves context cooperation with the
capability (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:131-138`). The production calls are
synchronous (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange.go:138-171`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/filestore.go:298-357`).

That ownership is honest. Preserve the rule wherever consumers invoke
extension capabilities: contain a synchronous panic at the ownership boundary,
do not format the panic value, and do not leak an unowned goroutine.

### 3. Stable error disclosure boundary

Unknown capability errors are collapsed to the Probe capability identity.
Trusted context cancellation and deadline identities are preserved with
`errors.Join`, while endpoint-bearing `url.Error` values and hostile error
types do not escape (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange.go:102-112`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange.go:174-199`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange_native_failure_test.go:26-64`).

That is stronger than forwarding arbitrary adapter errors into an operator
report. Preserve bounded typed projection at the actual owner, using
`errors.Is` and `errors.As`.

### 4. Closed bounded in-memory evidence

The archive uses closed enums for:

- outcome;
- failure identity;
- step kind; and
- directory state.

Reports retain at most eight fixed-array steps and return copies to callers
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/report.go:36-78`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/probe_constants.go:25`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange_hostile_test.go:234-247`).

The exhaustive enum tests sweep all 256 backing values and strict JSON
boundaries (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/enums_hostile_test.go:25-171`). The general pattern
is correct: bounded typed evidence is preferable to logs, loose maps, or
unbounded text.

### 5. Real primitive paths supplement the fakes

The native test:

- adapts actual `filestore.Write`, `filestore.Read`, and
  `filestore.RemoveFile` over a temporary directory; and
- adapts actual `exchange.FetchBytes` over a loopback HTTP server
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/native_integration_test.go:20-190`).

The Exchange failure matrix also drives real Exchange behavior through TLS
trust failure, injected DNS failure, retry exhaustion, and malformed content
type (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange_native_failure_test.go:19-114`).

That proof philosophy is worth retaining. Deterministic fakes may trigger
branches, but production-path evidence must cross the real implementation.

### 6. Explicit non-goals

The archive refuses daemon, uptime-monitor, scheduler, handler, discovery,
history, and benchmark scope. It also keeps the payload bounded to one MiB
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:140-175`). If a future concrete diagnostic is admitted,
these scope fences remain useful.

### Archived Primitive dependents

A complete committed production import search at archive HEAD found zero
Primitive packages importing `probe`.

The only real-primitive compositions are in Probe's own external test package:

- `nativeFileCapability` adapts Filestore
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/native_integration_test.go:20-88`); and
- `nativeExchangeCapability` adapts Exchange
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/native_integration_test.go:122-158`).

Those are test fixtures, not Primitive dependents. No application composition
root, package operation, command, handler, or persisted protocol consumes
either report. The package therefore has no downstream migration proof and no
evidence that its public report shape is the right one.

## Consumer evidence

### Kernel

Kernel does not import Probe. It has two distinct diagnostic surfaces whose
ownership should remain separate.

#### Store readiness and health

`api.Monitor` owns a narrow `Prober` interface whose actual capability is
`PendingCount(context.Context)`. Readiness invokes that operation under the
Core-owned two-second budget, marks readiness false on failure, and starts a
bounded recovery loop. Liveness does not depend on the store
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:20-45`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:215-257`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:320-377`;
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/api_contracts.go:66`).

Kernel separately exposes typed Health, Status, Ready, and Live request and
viewmodel types with `Validate` methods
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/api_probe_models.go:9-77`). The service mounts the four typed
routes and validates its delegated monitor
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:service/health/health.go:19-118`).

Local gem:

- distinguish liveness from readiness;
- let the dependency owner choose the real operation that demonstrates
  readiness;
- apply a bounded request deadline;
- make degraded state explicit; and
- keep recovery lifecycle and shutdown ownership in the service monitor.

This does not support a generic Primitive Probe. Kernel's readiness fact is
the result of a real store operation and product route policy, not a generic
directory artifact or Exchange correlation.

#### Read-only project doctor

`lfw doctor` validates a typed service through Alfred, checks compiled output,
and emits findings (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/doctor.go:13-94`). It is a useful
consumer-owned inspection command, but its filesystem checks, service set,
severity, and generated-contract rules are Kernel policy. They do not belong
in Primitive Probe.

### Witness

Witness does not import Probe. Its real diagnostic capability is the release
self-test and update failure protocol.

The candidate binary emits a typed `SelfTestResult` that binds:

- product;
- version;
- commit;
- platform;
- executable SHA-256;
- release authority key identity;
- server authority key identity;
- an ordered, complete typed check list; and
- success or an exact failed check
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/update_contract.go:309-428`).

Witness builds that fact from the embedded build and executable digest
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/build.go:122-146`). The updater executes the exact
candidate binary under a bounded self-test budget, retains output in a bounded
buffer, strictly decodes the typed result, and binds every identity field to
the signed target before acceptance
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/update.go:462-503`).

Failure evidence is built as a typed diagnostic, retained in one bounded
durable slot, delivered best-effort without masking the primary failure, and
deleted only after the corresponding signed receipt
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/diagnostic.go:14-109`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/store.go:17-87`).

Local gems:

- the diagnostic proves the exact production action, not a substitute socket;
- completeness is compiler-visible through the ordered typed check domain;
- result identity is bound to a signed expected target;
- hostile subprocess output is bounded before decode;
- the primary failure survives reporting failure;
- offline evidence storage is bounded to one validated record; and
- an exact signed receipt, including duplicate disposition, owns deletion.

These mechanics directly expose weaknesses in archived Probe: its correlation
is merely echoed by the same adapter that performed Exchange, its report has no
authenticated expected-target binding, and cleanup failure masks the primary
failure.

### Bug

Bug does not import Probe. It consumes the same typed release protocol and
implements a real standalone and update-path self-test.

`bug selftest --json` obtains the current embedded build, hashes the executing
binary, and emits the typed Primitive Release `SelfTestResult`
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/selftest.go:16-54`). `Build.SelfTest` binds the result to Bug,
version, commit, platform, binary digest, and both pinned authority identities
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/release.go:78-112`).

The update path executes only the compiler-owned self-test argv against the
absolute candidate path, uses an empty environment, and captures stdout through
the bounded typed output buffer
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/exec/exec.go:248-266`;
`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:478-499`). It then binds all returned fields to the signed
download target before replacing the installed binary
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:502-529`).

Failed candidate and installed self-tests are carried into the typed update
diagnostic and retained for retry until a signed receipt is verified
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:655-701`).

Local gems:

- invoke the exact downloaded artifact rather than an adapter;
- make argv compiler-owned;
- remove ambient loader, Git, shell, and toolchain environment influence;
- bind the returned fact to expected signed identity and content;
- distinguish candidate and installed self-test phases; and
- carry rollback outcome into durable failure evidence.

The operation is deliberately product-specific. A generic Exchange or
Filestore report cannot replace it.

### Peachfuzz

Peachfuzz does not import Probe. Its production foreground probe verifies the
actual governed Go toolchain before the daemon opens its evidence stores.

`ToolchainProbeSpec` owns the typed child Go environment and a nonzero typed
grace deadline with a `Validate` method
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/spec.go:90-107`). `Runner.ProbeGoToolchain`:

- validates context and request;
- resolves the compiler-owned Go command through the governor;
- derives the exact child environment;
- executes `go env GOVERSION`;
- bounds combined output through `process.Spec`; and
- parses stdout into `core.GoToolchainVersion`
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/run.go:18-54`).

On failure, `ToolchainProbeFailure` retains only bounded stderr, dropped-byte
counts, and exit code, validates those facts, and exposes the stable
`ErrGoToolchainProbeFailed` identity through `Unwrap`
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/toolchain_probe.go:11-53`).

Daemon assembly refuses to proceed if the probe fails and binds the observed
version into the child build context before opening the operational stores
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon.go:81-105`).

Local gems:

- run the exact capability whose availability matters;
- validate the typed execution spec at ingress;
- apply the production governor and child environment;
- retain bounded process facts, including dropped-byte evidence;
- parse success into a nominal version value; and
- make probe success a real DAG prerequisite.

This is the clearest counterexample to archived Probe's chosen domain.
Peachfuzz has a genuine product-neutral-looking foreground probe, but archived
Probe cannot express process execution at all. Adding a third generic
capability interface would broaden the package without creating a shared
semantic owner.

## Strong mechanics and proof

The archived strengths and consumer interviews above contain the located
mechanical and hostile proof for this package.

## Defects and blockers

### Verified defects and proof gaps

### 1. The adapters self-attest the facts Probe claims to prove

`FileStoreCapability` returns:

- directory safety state;
- bytes written;
- stream completion;
- file sync;
- atomic activation;
- directory sync;
- temporary removal;
- bytes read;
- target removal; and
- final directory sync
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/filestore.go:12-94`).

Probe validates these claims but has no independent source for most of them.
The native adapter demonstrates the problem:

- `StreamComplete` is hard-coded `true`;
- `FileSynced` is hard-coded `true`;
- both `Activated` and `DirectorySynced` are inferred from the same
  `filestore.ActivationDurable` value; and
- `TargetRemoved` and `DirectorySynced` are both inferred only from a nil
  removal error
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/native_integration_test.go:56-88`).

The hostile table proves only that Probe rejects a false boolean supplied by a
fake. It does not prove that a true boolean corresponds to a real durability
event (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/filestore_hostile_test.go:187-211`).

Exchange has the same structural problem. The target supplies the expected
correlation and the adapter returns that same value
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/native_integration_test.go:122-157`). This catches accidental
substitution inside an honest adapter but cannot establish end-to-end remote
correlation.

Finding: a report authored by the adapter under test is an assertion, not
independent conformance evidence.

### 2. Exchange byte counts are completely unvalidated

The specification says the observation proves `exact bytes sent and received`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:60-63`). `ExchangeObservation.Validate` checks only:

- correlation validity;
- attempts in `1..16`;
- status validity; and
- successful status class.

It never examines `BytesSent` or `BytesReceived`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange.go:36-59`).

The success fixture sets both fields to `math.MaxUint64`, and the minimum and
maximum success cases accept it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange_hostile_test.go:89-114`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange_hostile_test.go:291-310`).

This is direct falsification of a declared contract, not merely a missing
edge-case test.

### 3. Report validation does not own the report state machine

`reportState.Validate` validates only the first `count` slots. It does not
require unused fixed storage to be zero, constrain the ordered step sequence,
or relate step outcomes to the final outcome/failure
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/report.go:36-72`).

Consequently, the owning validator does not reject:

- nonzero trailing storage;
- successful reports containing failed steps;
- Filestore reports with impossible or repeated step order;
- a final failure unrelated to the failed step; or
- elapsed relationships inconsistent with the step set.

Private fields reduce external construction but do not make the invariant
compiler-owned. Internal construction, future decode, copying, and refactoring
still rely on the owner validator.

### 4. Failed Exchange reports retain evidence they do not validate

When a capability returns an error, classification ignores the returned
observation but `finishExchangeFailure` still stores it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange.go:74-78`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange.go:102-135`).

`ExchangeReport.Validate` validates evidence only on success
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/report.go:80-100`). A failed report may therefore validate
while exposing an arbitrary invalid `ExchangeObservation` through
`Evidence()`.

The type needs either:

- zero evidence on failure;
- a closed validated partial-evidence union; or
- no public evidence accessor for a failed state.

The archive provides none of those contracts.

### 5. Cleanup erases the primary failure identity

After commit or read/verify failure, `fileStoreRunner` stores one primary error
and failure identity, then performs cleanup
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/filestore.go:120-135`).

If removal or final inspection also fails, `cleanup` returns only
`FailureCleanup` and `ErrProbeCleanup`; the original capability or integrity
identity is discarded (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/filestore.go:211-219`).

The hostile suite tests each failure separately but has no combined
commit-plus-cleanup or integrity-plus-cleanup case
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/filestore_hostile_test.go:154-184`).

This is weaker than Witness and Bug, whose reporting failures never mask the
primary update failure. A truthful destructive-operation result needs a typed
primary outcome plus typed cleanup outcome, or an error graph preserving both
identities.

### 6. A late clock failure can erase completed external effects

The clock is observed before and after every step and again at report finish.
Any later observation error causes `execution.add` or `execution.finish` to
return an error without a report (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/report.go:121-175`).

That means:

- Exchange may complete successfully, then a clock failure returns a zero
  report;
- Filestore may commit bytes, then a clock failure prevents the step from
  being recorded; and
- cleanup may execute but the final report can still disappear.

The only panicking-clock test fails on the first observation and correctly
expects a zero report
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/exchange_hostile_test.go:152-221`). There is no clock that
succeeds initially and fails after an external effect.

This contradicts the specification's operational promise that once a valid
clock observation exists, operational failure returns a valid failed report
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:95-109`).

### 7. Required Filestore hostile proof is missing

The specification requires tests for:

- broad target roots;
- symlink roots;
- non-empty and unrelated initial entries;
- unreadable initial state;
- partial activation;
- Filestore cancellation;
- cleanup failure after a primary failure; and
- exact successful cleanup
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:149-165`).

The actual Filestore suite has four named tests covering payload-size extremes,
one-fault lifecycle cases, individual missing commit booleans, and
pre-inspection cancellation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/filestore_hostile_test.go:136-224`).

The native happy path checks only one ordinary empty temporary directory
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/native_integration_test.go:90-120`). No test source contains
the specified symlink, broad-root, unrelated-entry, unreadable, or combined
primary-plus-cleanup cases.

The archive ledger's assertion that those hostile paths are complete
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1030-1038`) is therefore stronger than the
proof.

### 8. The public API is a forbidden wrapper layer

Both exported operations are one-line wrappers around same-signature private
functions (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/public.go:7-15`).

Primitive 2026 explicitly rejects wrappers, aliases, shims, and compatibility
surfaces. If a future operation is admitted, the exported operation must own
the implementation directly.

### 9. Magic values and ownership are not compiler-clean

The deterministic payload byte is:

```go
byte((index*31 + 17) % 251)
```

The multiplier, offset, and modulus are raw magic values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/payload.go:41-43`).

Conversely, every Probe-specific token and local implementation limit was
moved into Core, including enum JSON size, payload buffer size, attempt count,
step count, and all report tokens
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/probe_constants.go:3-29`). No other production package imports
Probe and no archived dependent consumes those constants.

The 2026 ownership rule is:

- genuinely shared error identities and cross-package contracts belong in
  Core;
- package-local algorithm constants, enum tokens, and storage limits belong to
  their owning package; and
- raw numeric conventions belong nowhere.

The archive has both under-owned magic values and over-centralized local
contracts.

### 10. The evidence format is speculative

The specification says canonical report JSON and semantic fuzzing are deferred
until a persisted wire consumer exists
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:probe/SPEC.md:167-169`). No persisted or wire consumer exists, but
the archive already defines strict JSON for all report enums and Core-owned
wire tokens.

This creates protocol surface before demand while still omitting fuzz proof.
The correct 2026 response is not to add the missing fuzz target. The schema
should not exist until an owner and consumer exist.

## Primitive 2026 ownership and DAG

The admitted DAG should remain:

```text
core -> temporal
temporal -> exchange
temporal -> filestore

Kernel health/doctor -> its real store/filesystem operations
Witness update self-test -> release + exact candidate process
Bug update self-test -> release + exact candidate process
Peachfuzz toolchain probe -> runner + governed process
```

There should be no generic production node between the consumers and the
capability they must prove.

### Core owns

Core should own only contracts genuinely shared across admitted packages:

- base Primitive error identity;
- shared byte and path value types;
- shared HTTP status and method values;
- shared compiler-visible protocol tokens when more than one package consumes
  the same protocol; and
- cross-package validation rules.

Core should not predeclare Probe-specific:

- error identities;
- step tokens;
- failure tokens;
- directory-state tokens;
- artifact-name ceilings;
- attempt ceilings;
- payload limits; or
- report-storage limits.

No admitted package consumes those facts today.

### Exchange owns

Exchange owns:

- request and response facts;
- attempts;
- status;
- bytes actually written/read;
- transport and retry classification;
- redirect policy;
- context outcome; and
- its production-path integration proof.

If Exchange needs a foreground diagnostic operation, it should invoke its own
typed request directly and return an Exchange-owned validated result. A
consumer-specific correlation must be bound in the consumer's typed request
and response contract, not echoed through a Probe adapter.

### Filestore owns

Filestore owns:

- root and target safety;
- symlink resistance;
- streamed extent;
- durable commit;
- atomic activation;
- file and directory sync;
- temporary cleanup;
- read integrity;
- removal;
- residue inspection; and
- the strongest recoverable error graph.

Its native tests should incorporate Probe's O(1) deterministic artifact and
verifier pattern. A destructive runtime diagnostic, if required, remains in
the consumer that owns a dedicated target directory and cleanup policy.

### Consumers own

Consumers own:

- the operation that establishes readiness;
- product route and liveness/readiness policy;
- exact target identity;
- expected correlation semantics;
- process environment and governor;
- signed expected identity;
- diagnostic phase and check catalog;
- persistence, retry, receipt, and deletion policy;
- operator rendering; and
- the decision to block startup, update, deployment, or service readiness.

Those facts cannot be compressed into generic Outcome, FailureIdentity, and
Step without losing the contract that makes the evidence meaningful.

### Admission trigger for any future shared diagnostic package

Do not reopen admission merely because another package uses the word `probe`.
A future shared capability must first demonstrate all of the following:

1. at least two production consumers perform the same product-neutral typed
   operation;
2. one package can own the operation without adapters self-authoring proof;
3. target identity and expected result are compiler-visible;
4. success and partial failure facts have complete owner validation;
5. primary and cleanup outcomes remain separately typed;
6. execution is bounded and O(1) where data size varies;
7. the production path, not only a fake interface, is tested;
8. a real caller migrates before the public result schema freezes; and
9. no package-specific constants or error prose are copied into consumers.

Until that evidence exists, the strongest and smallest Primitive is the one
without `probe`.

## Decision rationale and conditions

### Recon implications

`REJECT ARCHIVED PACKAGE`

Do not add `foundation/probe`, do not copy its source, and do not preserve its
public API or Core contract surface.

Retain these gems at the actual owner:

- deterministic bounded payload generation;
- exact streaming extent and digest verification;
- fixed-capacity typed evidence with closed validators;
- synchronous panic containment;
- trusted cancellation/deadline identity;
- hostile external-error disclosure collapse;
- native production-path tests; and
- explicit non-goals against monitoring and background world-building.

Before any of those mechanics are reused, correct the archive defects:

- type the payload algorithm constants;
- close every unused fixed-storage slot;
- validate the complete step state machine;
- never accept unvalidated byte counts;
- never retain invalid failed-state evidence;
- preserve primary and cleanup identities together;
- preserve evidence after late instrumentation failure; and
- prove every declared hostile path.

The consumer reports are already stronger because they execute the exact
operation and bind the result to facts the consumer owns. Primitive 2026
should strengthen those real paths through Exchange, Filestore, Release,
Temporal, Contextstate, and Process contracts. It should not add an
adapter-driven universal probe.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
