# Testserial package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `testserial`.
The archive is evidence, not authority. No archived production or test source
was copied.

There is a real shared need. Primitive itself has seven tests that must not
call `t.Parallel`; Kernel and Witness use the typed declaration pervasively,
and Peachfuzz uses it in allocation, signal, external-service, ordering, and
package-seam tests. The archive also made important advances over comment
waivers: the reason is a closed Core enum, invalid values preserve a stable
Core-owned error identity, production imports are ratcheted, and Witness-lint
recognizes the exact package, function, owner, and Core constant.

The archive is nevertheless not admissible unchanged. `Serial` does not
serialize anything. It validates a reason and relies entirely on the caller
omitting `t.Parallel` plus an external source analyzer. Worse, the archive's
own only package test calls both `t.Parallel()` and `Serial`, and Witness-lint
accepts that contradiction because the serial check wins before the parallel
check. The analyzer also accepts a declaration hidden in a conditional,
unreachable branch, or nested function literal because it searches the whole
AST without a control-flow or statement-position contract.

The 2026 capability should be reconstructed as a compiler-visible test
isolation declaration, not represented as a lock and not described as runtime
serialization. It should own:

1. one validated Core-owned declaration structure;
2. closed, generic hazard and scope enums;
3. a single direct declaration function for `*testing.T`;
4. stable Core-owned contract errors; and
5. the exact Core-owned package/function/analyzer contract required for
   Witness-lint conformance.

It must not own Go scheduling, a mutex, a file lock, process state, an external
service, consumer-specific assets/pages/seams, benchmark or fuzz locking, or
JSON persistence. Actual cross-process locking is a separate capability.

Admission remains blocked until the analyzer rejects contradictory, hidden,
late, duplicate, and unsafe descendant declarations; the helper boundary has
non-vacuous invalid-value proof; the reason model loses consumer-specific
catch-alls; and the Primitive 2026 module path is supported by the exact
pinned Witness-lint generation.

### Exact source revisions and Primitive pins

| Source | Exact revision or Primitive pin | `testserial` evidence | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`, `Harden capability inventory evidence`) | Tree `76fa530f071081db988f6cf208a0e0f6ebd02ef4` | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; the inspected package and Core files are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`; committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins Primitive `0df2954a2d911a5d7d775691d023d569affa2c20` (`2026-07-22T21:25`, `2026-07-22T21:01-04`, `2026-07-22T21:00`) | Committed tree `20a2015bfdc7662e25d4b68fef75722342ee4675`. Dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` (`2026-07-27T00:33`, `2026-07-27T00:47-04`, `2026-07-27T00:00`), tree `76fa530f071081db988f6cf208a0e0f6ebd02ef4`. | Broad pre-existing dirty migration. Committed and dirty pins are separate evidence. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e`; `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins Primitive `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` (`2026-07-22T07:30`, `2026-07-22T07:53-04`, `2026-07-22T07:00`) | Tree `20a2015bfdc7662e25d4b68fef75722342ee4675` | Only the pre-existing untracked `.ledger_pending.md` was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc`; `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins Primitive `388e593231a28434f6faae9f0ab9dffcf332dfc3` (`2026-07-20T10:59`, `2026-07-20T10:21-04`, `2026-07-20T10:00`) | Tree `1f76fa1318d7968b44dac1c9d98111ed775aeb6f`; no active import or call | Only the pre-existing untracked `.ledger_pending.md` was observed. Vendored and `_foundation_source` copies are evidence snapshots, not adoption. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a`; `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins Primitive `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` (`2026-07-23T17:51`, `2026-07-23T17:17-04`, `2026-07-23T17:00`) | Tree `20a2015bfdc7662e25d4b68fef75722342ee4675` | Only the pre-existing modified `.ledger_pending.md` was observed. |

The package history material to this decision is:

- `bfb8f6468e0701f4f4b5f2d26c8e4188b13aac00`: introduce the Core enum,
  helper, and initial test;
- `a20301f5417238f0f6ec8e58e04c92b93cbdcaa8`: add shared compiled-pages
  and package-seam reasons and ratchet Witness-lint;
- `4a57cd1e808c843b4fe312386600bfba9bb37125`: add doctrine ownership;
- `c11c22e53ab6c6cef1b4cd70c1e67620c7e58151`: add the compiler witness for
  the helper signature; and
- `40ded9c104a99cbc4b0b672cd7392901b468d1eb`: add the final eight-line
  package specification.

The four consumer pins contain the same behavioral helper. Archive HEAD mainly
adds the final specification and signature witness. No compatibility surface
is required: Primitive 2026 can make a clean break and update real call sites.

### Archive census and fresh focused proof

Archive HEAD contains exactly four files and 75 lines:

| File | Lines | Role |
| --- | ---: | --- |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/SPEC.md:1-8` | 8 | ownership claim |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/doctrine_contract.go:1-13` | 13 | Primitive/TestSupport doctrine declaration |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/serial.go:1-22` | 22 | helper and exact signature witness |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/serial_test.go:1-32` | 32 | positive reason enumeration |

A clean extraction of the exact archived HEAD passed:

```text
go test ./testserial ./core
```

This focused run proves that the package and its Core contracts compile and
that the current deterministic tests pass without relying on the unrelated
untracked archive file. It does not prove runtime serialization, analyzer
soundness, invalid-value rejection by the helper itself, or ownership truth.

## Capability ownership

The specification says `testserial` is test-only support for compiler-visible
serialization reasons. `Serial` accepts one `testing.TB` and one closed
`core.TestSerialReason`, calls `Helper`, validates the reason, and calls
`Fatal` on validation failure
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/SPEC.md:3-8`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/serial.go:12-19`).

Its explicit nonownership is correct and important:

- no lock;
- no scheduler;
- no process state; and
- no production behavior
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/SPEC.md:3-6`).

The exact function shape is compiler-witnessed:

```text
func(testing.TB, core.TestSerialReason)
```

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/serial.go:21`).

Core owns the closed `uint8` reason enum, zero invalid value, private limit,
validation, token and Go-identifier projections, parsers, and JSON methods
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/testing_contracts.go:8-132`). Core also owns:

- `ErrTestSerialContract`, wrapping `ErrPrimitiveContract`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:141`);
- the error format (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_constants.go:298`);
- the exact Primitive package path
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_constants.go:71`); and
- the exact Go package/function naming contracts used by the analyzer
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_constants.go:71-76`).

That is the right SSOT direction: callers do not invent reason strings,
package paths, function names, or stable error prose.

## Archive evidence

### Typed declaration instead of comment folklore

The strongest archive decision is replacing waiver comments with a call the Go
compiler type-checks. The project testing protocol explicitly says Kernel tests
must use `testserial.Serial` with a Primitive-owned reason; invalid/zero
reasons fail at execution and Witness-lint verifies the exact import and
function contract
(`foundation@working-tree:_docs/testing_protocol.md:1370-1374`).

The protocol also truthfully treats the requirement as the alternative to
`t.Parallel`, not as a runtime lock
(`foundation@working-tree:_docs/testing_protocol.md:277-298`).

### Closed values and stable error identity

`TestSerialReason.Validate` rejects zero and out-of-range values while
preserving both `ErrTestSerialContract` and `ErrPrimitiveContract` through
wrapping (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/testing_contracts.go:82-91`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/testing_contracts_test.go:32-86`). Tests use `errors.Is`, not
error-string matching.

Core's hostile table covers zero, the private limit, maximum `uint8`, every
valid reason, token projection, and Go-identifier round trips. Separate tests
reject identifier lookalikes, exercise malformed JSON and the closed JSON
set, and fuzz all `uint8` inputs
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/testing_contracts_test.go:32-171`).

### Exact analyzer identity

Witness-lint does more than grep. It verifies:

- the exact Primitive testserial import path;
- the exact Core package path;
- the exact helper selector;
- exactly two arguments;
- the current test/subtest variable as the first argument; and
- a known Core reason constant as the second argument
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/test_protocol.go:411-455`).

Its hostile fixture table rejects a lookalike package, wrong function, other
test owner, string reason, local lookalike, renamed helper, and unknown Core
identifier
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/doctrine_test.go:3037-3168`).

These checks are useful 2026 evidence. They should be strengthened, not
discarded.

### Production-import ratchet

Primitive scans non-test Go files structurally and rejects imports of
test-only packages including `testserial`. It skips `.git`, vendor, and
testdata and includes a minimum scanned-file floor so a wrong root cannot
produce a vacuous pass
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/test_support_architecture_test.go:13-72`).

No active production import was found in Primitive, Kernel, Witness, Bug, or
Peachfuzz.

### Primitive's own dependents

Archive HEAD contains seven direct calls in six package test suites:

| Consumer | Reason | Behavior actually protected |
| --- | --- | --- |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/allocation_hostile_test.go:10-37` | `RuntimeAllocation` | allocation count remains extent-independent |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/stream_hostile_test.go:191-209` | `RuntimeAllocation` | stream-copy allocations remain independent of extent |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/bounded_reader_hostile_test.go:615-645` | `RuntimeAllocation` | open/read allocations do not scale with extent |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/transfer_hostile_test.go:1098-1125` | `RuntimeAllocation` | exact upload/hash allocations do not scale with source size |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/wire_hardening_test.go:356-375` | `RuntimeAllocation` | wire copying stays allocation-bounded |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate_hostile_test.go:501-530` | `RuntimeAllocation` | artifact evaluation allocations stay extent-independent |
| `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/signal_native_unix_test.go:47-65` | `ProcessSignal` | a real subprocess follows the native signal lifecycle |

These are legitimate hazards. Allocation measurements read runtime-global
allocation state; the signal test intentionally exercises a process seam.
They support admitting a narrow generic declaration contract.

They do not support the archive's consumer-specific reasons for compiled
assets, compiled pages, or a generic package seam.

## Consumer evidence

### Kernel

At committed HEAD, Kernel contains 117 textual direct calls across 44 files.
The breadth is real: process output, logger, environment, working directory,
signals, allocation probes, registries, compiled assets/pages, external
integration, ordered state, and package seams.

Representative evidence includes:

- output capture
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:alfred/regression_test.go:22`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:alfred/regression_test.go:74`);
- native signal handling
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/server_signal_unix_test.go:38`);
- environment and working-directory mutation
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/config_test.go:628`;
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:compass/port_test.go:151`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:compass/port_test.go:183`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:compass/port_test.go:199`);
- allocation observation
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/api_test.go:266`); and
- shared compiled assets throughout the Ballast suite
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:ballast/ballast_test.go:1-654` and
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:ballast/bench_test.go:1-942`).

Kernel adds two useful pieces of evidence.

First, `sentinel/TestGoTestsUseTypedSerialContracts` structurally scans test
files and forbids the Core-owned legacy comment markers
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:sentinel/test_serial_contract_test.go:17-72`). This is a useful local
ratchet, though Witness-lint remains the authority for requiring either a
parallel call or typed declaration.

Second, Kernel owns a real advisory `flock` facility for tests, benchmarks,
and fuzz harnesses, including reentrant ownership
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:internal/testlock/testlock.go:15-148`). That capability is not a
replacement implementation for `testserial`: it supplies actual
cross-process/resource exclusion that the Primitive declaration explicitly
does not own. Its consumer demand should be interviewed separately before any
generic lock is considered. Its current raw lock-name and path conventions are
not admissible into 2026 Primitive as-is.

Kernel also exposes an actual quality gap: one test declares the same
`PackageSeam` reason twice on adjacent lines
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/keys_test.go:393-395`). The helper silently accepts duplicate
declarations, and the analyzer has no exactly-once rule.

### Witness

Witness has 126 textual direct-call references across 35 files at committed
HEAD. Several are intentionally embedded in analyzer fixture strings, so the
number is not a count of executed tests. Real executed use is still broad:
working-directory mutation, environment mutation, signals, shared registries,
runtime allocation, ordered state, external services, and package seams.

Representative direct consumers include:

- Witness-lint command working-directory tests
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-lint/main_test.go:13`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-lint/main_test.go:56`);
- crash harness signal tests
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/run/crash_harness_test.go:225`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/run/crash_harness_test.go:483`);
- CLI environment and registry tests
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/cli/cli_test.go:1728`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/cli/cli_test.go:2196`); and
- execution/verification seams using `PackageSeam` and `OrderedState`.

Witness is also the semantic implementation dependency. Primitive's helper
does nothing to Go scheduling; correctness depends on the pinned analyzer
recognizing the exact Primitive generation. The archived disabled doctrine
workflow pins Witness-lint commit
`670642153febca8179b47fe96c7b3b61d55db3e4`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.github/disabled-workflows/doctrine.yml:36-46`).

This external pin is a real cross-repository contract. The 2026 package cannot
be admitted until the exact installed Witness-lint understands the 2026
Primitive module path and strengthened declaration shape. No alias for the
2026 path is acceptable.

### Bug

Bug has zero active direct calls and zero active imports at committed HEAD.
Its vendored Primitive files and `protocol/_foundation_source` snapshots do
not prove consumer demand. Bug therefore contributes an important neutral
result: 2026 must not enlarge this package to solve an imagined Bug use case.

### Peachfuzz

Peachfuzz has 15 direct calls across seven files at committed HEAD:

- seven runtime-allocation declarations;
- four ordered-state declarations;
- two external-service declarations;
- one process-signal declaration; and
- one package-seam declaration.

Representative evidence:

- live GCS tests
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/gcs_live_test.go:15`;
  `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs_live_test.go:24`);
- real process-signal behavior
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/process/run_test.go:237`);
- runner allocation probes
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/run_test.go:85-249`); and
- worktree ordering
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree_test.go:229`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree_test.go:265`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree_test.go:527`).

Peachfuzz also demonstrates why scope must be typed. A top-level test calls
`t.Parallel`, then its subtests call `Serial(...OrderedState)` while sharing
one parent-owned marker path
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:cmd/peachfuzz/main_test.go:42-77`). That case may be safe because
the intended ordering is only among siblings and the path is unique to the
parent. A process-environment or signal declaration in the same parallel
ancestor shape would not be safe. The archive has no type-level way to
distinguish those scopes, and Witness-lint does not check ancestor safety.

## Strong mechanics and proof

The archive proves the value of a typed test isolation declaration, a closed
Core-owned reason domain, stable error identity, exact analyzer matching, and a
production-import ratchet. Kernel, Witness, and Peachfuzz provide independent
demand. The focused archive test run proves current compilation and deterministic
behavior, while the consumer evidence identifies the generic hazards and scope
relationships the archived declaration does not represent.

## Defects and blockers

### 1. The name overstates the runtime behavior

`Serial` cannot make a test serial. It performs no scheduling call or lock; it
only validates an enum (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/serial.go:12-19`). Actual Go test
behavior comes from not calling `t.Parallel`, and policy enforcement comes
from Witness-lint.

The specification's nonownership text is truthful, but the exported verb is
not. The 2026 name should describe declaration, for example `Declare`, and its
specification must say explicitly that it provides no mutual exclusion.

### 2. The archive proves and accepts a contradictory contract

The only package test calls `t.Parallel()` at line 10 and then calls `Serial`
for every reason at lines 29-30
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/serial_test.go:9-31`).

Witness-lint accepts this because `topLevelTestParallelViolations` returns
success immediately when it finds a serial call, before checking for
`t.Parallel`
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/test_protocol.go:331-343`).

This is not a hypothetical bypass. The archive's canonical positive test
contains it. A test must have exactly one isolation mode.

### 3. The analyzer accepts hidden or unreachable declarations

`hasTestSerialContract` walks the complete body with `ast.Inspect` and accepts
the first matching call anywhere
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/test_protocol.go:411-425`). Unlike the subtest
collector, it does not stop at nested function literals. It does not establish
that the declaration is:

- a direct statement in the test body;
- unconditional;
- reachable;
- before the behavior under test; or
- unique.

A call inside `if false`, after a return, or inside an uncalled function
literal can therefore satisfy the analyzer. The existing hostile table does
not cover these cases
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/doctrine_test.go:3037-3168`).

The 2026 analyzer must recognize a strict declaration position and reject
both zero and multiple declarations. It must reject a simultaneous
`t.Parallel` call anywhere in the same test or subtest.

### 4. Parent/child safety is not represented

Top-level and subtest checks are independent
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/test_protocol.go:331-408`). The analyzer does not
propagate a parallel ancestor into the child's isolation decision.

Some reasons are process-wide and cannot be safely used beneath a parallel
ancestor. Other reasons may express only deterministic sibling order over
parent-private state. A flat reason enum cannot encode this distinction.
Primitive 2026 needs a validated scope field, and Witness-lint must enforce
ancestor compatibility.

### 5. The reason set contains consumer policy and catch-alls

The archive enum includes `SharedCompiledAssets`, `SharedCompiledPages`,
`ExternalService`, `OrderedState`, and `PackageSeam`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/testing_contracts.go:14-28`).

The first two are overwhelmingly Kernel concepts and have no Primitive
dependent. `PackageSeam` dominates consumer use but does not identify any
specific shared hazard. `ExternalService` does not inherently require serial
execution: distinct accounts, projects, or emulators may be independent.
`OrderedState` does not say whether the scope is one sibling table, one
package, one process, or an external resource.

These values act as compiler-visible labels but not validated contracts.
Consumer-specific assets/pages/seams must remain consumer-owned. The shared
Core contract should model generic hazards and scope, not copy consumer nouns
into Primitive.

### 6. The helper-boundary test is vacuous for validation

The sole package test validates each reason directly before passing it to
`Serial`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:testserial/serial_test.go:24-30`). If `Serial` stopped calling
`Validate`, that test would remain green. It never sends zero or a future enum
value through the helper boundary.

Core proves the enum well, but it does not prove that the helper enforces its
ingress boundary. Because `Fatal` terminates the calling goroutine, the helper
needs a hostile subprocess or equivalent real `testing` boundary test. The
current test name also leaks the product token `OGS` into a product-neutral
package.

### 7. The public surface is broader than evidence requires

All real consumers call the declaration with a Core constant. The search found
no active consumer of the JSON representation or token parser. Witness alone
uses `ParseTestSerialReasonGoIdentifier` to analyze source
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/test_protocol.go:441-455`).

`MarshalJSON`, `UnmarshalJSON`, and `ParseTestSerialReason` therefore create a
persistence/wire protocol for a source-level test declaration without an
observed owner or consumer
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/testing_contracts.go:94-132`). They should not be copied into
2026. The analyzer-facing identifier projection/parser may remain only if the
strengthened analyzer still needs it.

The `testing.TB` parameter is similarly broader than observed use. All direct
calls reviewed are test/subtest declarations, while benchmarks and fuzz
harnesses have different lifecycle and locking requirements. A `*testing.T`
parameter makes the admitted contract narrower and more compiler-visible.

### 8. The specification and architecture proof are incomplete

The eight-line specification names the stable surface and production-import
ban, but it does not fully state:

- the declaration's execution and failure contract;
- scope/ancestor behavior;
- exactly-once and mutual-exclusion rules;
- dependencies and external analyzer pin;
- resource and allocation bounds;
- platform behavior;
- stable error identities; or
- hostile proof obligations.

There is also no package-local public-surface ratchet proving that only the
intended declaration function is exported. The production-import scan is
strong Primitive-local proof, but downstream production import prohibition
still depends on each consumer's governance gate.

## Primitive 2026 ownership and DAG

### Admit

Reconstruct a small test-support package whose sole operation is a
compiler-visible isolation declaration.

Core should own:

- the exact Primitive 2026 package path and declaration function name;
- `TestIsolationHazard`, a closed enum containing only generic hazards with
  demonstrated shared demand;
- `TestIsolationScope`, a closed enum that distinguishes at least test,
  sibling-table, package-process, and external-resource boundaries where those
  distinctions are enforceable;
- `TestIsolationDeclaration`, a typed struct containing the hazard and scope;
- `Validate()` on each enum and on the aggregate declaration;
- stable `ErrTestIsolationContract` identity wrapping the Primitive contract;
  and
- any analyzer-facing Go-identifier projection that has an actual Witness
  consumer.

The package should own one direct function, conceptually:

```text
Declare(*testing.T, core.TestIsolationDeclaration)
```

The exact final identifier is a design decision for reconstruction, not a
compatibility promise from this report. The function must call `Helper`,
validate at ingress, and fail with the typed Core-owned identity. It must not
claim to acquire a lock.

Initial generic hazards should be admitted only from demonstrated shared
needs, such as process environment, working directory, signal handlers,
process output/logger, global registry, runtime allocation observation, and
ordered sibling state. Each must have a precise scope rule. Values should not
be added merely to preserve 2026 names.

### Keep out

Do not admit:

- `Serial` as a compatibility wrapper or alias;
- raw reason strings;
- JSON marshal/unmarshal or token parsing without a persistence consumer;
- `SharedCompiledAssets`, `SharedCompiledPages`, or `PackageSeam`;
- a generic `ExternalService` waiver without a typed resource/scope contract;
- runtime mutex/file-lock behavior;
- Kernel's local lock names or fixed path convention;
- benchmark/fuzz APIs hidden behind `testing.TB`; or
- copied 2026 module paths and analyzer conventions.

### Analyzer admission requirements

The exact pinned Witness-lint must:

1. type-check or otherwise prove the exact Primitive 2026 import and typed
   aggregate declaration;
2. require exactly one direct, unconditional declaration in an enforced
   statement position;
3. reject a declaration combined with `t.Parallel`;
4. reject declarations in branches, loops, closures, deferred calls, or
   unreachable code;
5. enforce ancestor compatibility using the typed scope;
6. reject unknown, zero, incomplete, or consumer-local lookalikes;
7. preserve exact Core-owned rule and error identities in machine output; and
8. carry hostile fixtures for every bypass above.

The Primitive gate must install an exact Witness revision that implements
this 2026 shape. The helper and analyzer are one delivery contract even though
they remain separate packages and repositories.

### Required hostile proof before admission

The reconstructed package and Core contract need at least:

- a table covering zero, private limit, maximum backing value, and every valid
  hazard and scope;
- aggregate `Validate()` tests for every zero field, invalid combination, and
  valid boundary combination;
- `errors.Is` proof for both the isolation identity and Primitive identity;
- a real helper-boundary test proving invalid declarations fail before test
  behavior executes;
- exact public function-signature witness;
- package public-surface and production-import ratchets with non-vacuous scan
  floors;
- analyzer fixtures for valid top-level and valid scoped subtest declarations;
- analyzer rejection fixtures for both modes, duplicates, wrong owner,
  lookalike import/function/type, zero or unknown constants, conditional,
  looped, nested-closure, deferred, late, and unreachable declarations;
- ancestor-scope fixtures proving process-global hazards cannot hide beneath a
  parallel parent while safe parent-private sibling ordering remains possible;
- focused race and repeated runs where relevant;
- canonical Primitive gate proof; and
- focused Kernel, Witness, and Peachfuzz migrations using the new contract.

Tests must use the typed declaration and Core constants themselves. They must
not assert analyzer prose or duplicated source tokens when a Core-owned
identity exists.

## Decision rationale and conditions

The capability warrants narrow admission only after redesign. The archived
implementation and reason set are rejected as-is.

The compiler-visible declaration idea is worth preserving. Its direct
Primitive dependents and broad Kernel/Witness/Peachfuzz use prove shared
demand, and the Core enum/error/path ownership plus exact analyzer matching are
genuine strengths.

Admission is not yet complete because the current package/analyzer pair accepts
its own contradictory `t.Parallel` plus `Serial` test, accepts hidden or
unreachable declarations, cannot validate parent/child scope, exposes
consumer-specific catch-alls, and has a vacuous helper-boundary test.

The clean 2026 outcome is a smaller and stricter package: typed aggregate
declaration, generic hazards, explicit scope, exact analyzer conformance, no
runtime-lock pretense, no JSON world-building, no compatibility layer, and no
consumer nouns in Primitive Core.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
