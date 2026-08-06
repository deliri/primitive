# Testing Protocol

This protocol is not a ceremony and not a separate truth source.

It is the test-quality doctrine for evidence-producing Go pipelines. Rules
that can be proven from source belong in a lint tool. Rules that require
intent stay in this document until they can be encoded safely. The
references to `witness-lint` and the `witness:waiver` comment grammar
throughout this document are this codebase's implementation of the rule;
adopt the same names or substitute equivalents that fit your project.

The goal is not pretty tests. The goal is evidence.

In this protocol, "hostile" means extreme valid and invalid inputs on both
sides of every expected boundary, plus malformed data, partial results,
cancellation, resource failure, and state-transition pressure. It describes
test intensity, not a threat model or an assumption about malicious callers.

## Rule Model

Each rule has:

- `id` — stable name used by docs, waivers, and future lint output
- `level` — `must`, `should`, or `review`
- `enforcement` — `lint`, `tool`, `review`, or `future-lint`
- `why` — the failure class it prevents

`must` means a violation is a correctness risk. `should` means the default is
required unless there is a better local reason. `review` means the rule needs
human judgment today.

When rules conflict, the more specific rule wins. A serial test may override
`test/parallel/default` only with a waiver. A tempdir isolation rule may override
a helper convenience pattern. A structural invariant test may use source
inspection when a runtime test cannot prove the compiler-visible contract.
When two rules still conflict, choose the rule that preserves evidence
accounting and determinism. If that choice weakens another rule, add a waiver
at the call site and state the exact failure class being accepted.

## Rule Index

| id | level | enforcement | summary |
| :- | :---- | :---------- | :------ |
| `test/review-checklist` | must | review | Reviewer questions for every new or changed test |
| `test/compiler-driven` | must | review | Keep the compiler in charge; clean break over compat shims |
| `test/evidence` | must | review | Tests must prove a behavior that can fail |
| `test/red-green-slice` | must | review | Every slice has a red state or a named ratchet reason |
| `test/isolation/tempdir` | must | lint | Filesystem-mutating tests own a tempdir per parallel unit |
| `test/parallel/default` | should | lint | Tests parallelize by default; serial cases need a waiver |
| `test/sync/no-sleep` | must | lint | Synchronize on facts, not on `time.Sleep` |
| `test/determinism` | must | review | Control time, randomness, paths, env, IDs, command output |
| `test/table-shape` | must | review | The table is the pressure surface; names name the boundary |
| `test/boundaries` | must | review | Policy/data code needs negative and boundary coverage |
| `test/layer-triad` | must | review | Each touched layer needs positive/negative/neutral coverage |
| `test/package-sweep-exit` | must | review | A sweep is done when surfaces have coverage and legacy retired |
| `test/errors` | must | review | Assert the strongest stable error contract (`errors.Is`/`As` over strings) |
| `test/structural-equality` | must | lint | No `reflect.DeepEqual`; use `==`, `slices.Equal`, or explicit fields |
| `test/helpers` | must | lint | No testify/`assert*`; show `got`/`want` in plain Go |
| `test/fixtures/no-shared-mutable` | must | lint | Parallel tests must not share mutable fixtures |
| `test/fixtures/non-vacuous` | must | review | Fixtures must force the behavior under test to execute |
| `test/production-path` | must | review | Be honest about which path the test exercises |
| `test/evidence-path-consistency` | must | review | Ref-says-X ∧ manifest-says-X ∧ disk-has-X for every evidence file |
| `test/goroutines/owned` | must | review | Every goroutine has owner, cancel path, wait path, timeout backstop |
| `test/structural-invariant` | must | review | Source/AST tests valid when contract is compiler-visible structure |
| `test/data-flow-inventory` | must | review | Production structs in eligible packages must be classified |
| `test/repeat-policy` | must | review | `RepeatCount` is tests-only; tools/benchmarks/fuzz run once per phase |
| `test/budget-divergence` | must | review | Effective/configured budget ratio is a contract, not a side effect |
| `test/benchmarks` | must | review | Measure one thing, report allocs, run serial+last, manifest covers evidence |
| `test/fuzz-boundary` | should | review | Fuzz at trust boundaries; evidence covered by manifest |
| `test/ledger-chain` | must | review | Ledger tests prove the real chain |
| `protocol/typed-boundary` | must | lint | Protocol payloads are typed structs and enums |
| `test/waivers` | must | review | Waivers name why the rule is wrong for this case |

Grep this index by `id` to jump to the rule body. Every rule body restates the
`id`, `level`, and `enforcement` line so the table stays a navigation aid, not a
duplicate truth source.

## Review Checklist

`id: test/review-checklist`

`level: must`

`enforcement: review`

Review-only rules are still rules.

For every new or changed test, reviewers must ask:

- What behavior would fail red before the fix?
- Which production contract is being ratcheted: parser, validator, classifier,
  fold, writer, manifest, verifier, reporter, CLI, or doctrine?
- Does the test own filesystem mutations with `t.TempDir()` in the test or
  parallel subtest body?
- Is the test parallel, or does it carry a canonical waiver?
- Are time, randomness, paths, environment, IDs, and command output controlled?
- For tables, do case names describe boundaries or failure modes?
- For policy/parser/classifier/fold code, are negative and boundary cases
  present?
- For evidence/accounting changes, does every touched layer have positive,
  negative, and neutral coverage?
- Are errors asserted with sentinels, typed errors, or a documented waiver?
- If a value is derived, does the test prove the source facts rather than
  restating a persisted duplicate?
- Do helpers preserve `got`/`want` facts instead of hiding assertions?
- Do fixtures force the behavior under test to execute, instead of passing by
  no-op?
- If the test scans source or reflects on types, is the invariant structural and
  tied to a real bug class?
- If a production struct was added, is it covered by the package's struct or
  data-flow inventory and classified into an intentional role?
- If old tests support a deleted legacy path, were they upgraded to the new
  contract instead of kept as compatibility ballast?

## Compiler-Driven Testing

`id: test/compiler-driven`

`level: must`

`enforcement: review`

Tests must keep the compiler in charge.

The preferred upgrade path is a clean break: structs to structs, enums to
closed `iota` enums, explicit `Validate()` gates, typed sentinels, and direct
call sites. Do not add compatibility wrappers, shims, alternate model structs,
or stringly adapters just to keep old tests green.

When a legacy assumption is wrong, upgrade the assumption. Do not fight the old
test. Either rewrite it around the new typed contract or replace it with a
non-vacuous test that proves the current execution path.

Required:

- public protocol data is a typed struct or closed enum
- derived values are tested through their source facts and method result
- invalid states are rejected by `Validate()` or a boundary constructor
- errors route through typed sentinels or typed error structs
- tests assert the compiler-visible shape when shape is the contract

Forbidden:

- test-only wrappers that hide production shape changes
- compatibility constructors that preserve deleted fields
- duplicate fixtures that encode the same truth in two places
- `map[string]any` or raw JSON fixtures where a typed fixture is available
- keeping an old substring/prose assertion when a sentinel exists

If the compiler cannot see the contract, first ask whether the production code
needs a typed surface. Runtime assertions are still needed, but they should sit
on top of compiler-visible structure, not replace it.

## First Principle

`id: test/evidence`

`level: must`

`enforcement: review`

A test must prove a behavior that can fail.

If a test cannot explain what bug it catches, it is theater. Delete it or
rewrite it. A useful test has a meaningful red state, then proves the green
state after the fix.

Required proof:

- the assertion checks a real contract, not just that code executed
- failure output identifies the broken fact
- the test would fail if the production behavior regressed
- the fixture cannot pass by skipping the production branch under review
- the test is connected to the actual production path unless the slice is
  explicitly a direct unit ratchet for a helper

Weak:

```go
func TestBuild(t *testing.T) {
	t.Parallel()
	_ = Build()
}
```

Strong:

```go
func TestBuildRejectsMissingCommand(t *testing.T) {
	t.Parallel()

	_, err := Build(Input{})
	if !errors.Is(err, ErrMissingCommand) {
		t.Fatalf("Build() error = %v, want %v", err, ErrMissingCommand)
	}
}
```

## Red/Green Slice Discipline

`id: test/red-green-slice`

`level: must`

`enforcement: review`

Every behavioral slice should have a red state before the production fix or an
explicit reason why the slice is a pure ratchet over already-correct behavior.

Acceptable slice shapes:

- bug fix: add or expose the failing hostile case, fix production, keep the
  hostile case as a ratchet
- contract ratchet: production is already correct, but the test now proves a
  previously unpinned invariant
- deletion/collapse: flip tests away from the deleted legacy field/path and
  assert absence where the wire or struct shape matters
- structural guard: source/AST test proves a compiler-visible invariant that a
  runtime test cannot observe cheaply

Unacceptable slice shapes:

- green test that exercises only a fake path while claiming production coverage
- fixture pre-writing a file that production cannot see at that point in the
  lifecycle
- test that pins a compatibility branch scheduled for deletion
- assertion that duplicates production logic without independently checking the
  output state

When a slice intentionally stops short, name the remaining gap in the commit or
review note. "Out of scope" must identify the next proof surface, not hide it.

## Isolation

`id: test/isolation/tempdir`

`level: must`

`enforcement: lint`

Every test that mutates the filesystem must own its directory.

If a test or subtest calls `t.Parallel()` and creates, writes, renames, removes,
links, chmods, or truncates files, it must call `t.TempDir()` in the same test or
subtest function body.

Parent temp directories do not make child subtests safe. A parallel subtest is a
separate unit of isolation.

A helper may receive a temp directory created by the test or subtest. A helper
must not hide `t.TempDir()` for a parallel subtest that mutates files; ownership
must be visible at the call site.

Wrong:

```go
func TestWrite(t *testing.T) {
	t.Parallel()

	err := os.WriteFile("ledger.jsonl", []byte("{}"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
```

Right:

```go
func TestWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	err := os.WriteFile(path, []byte("{}"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
```

## Parallelism

`id: test/parallel/default`

`level: should`

`enforcement: lint`

Tests should call `t.Parallel()` at the parent and subtest level unless they
mutate process-wide state or intentionally prove ordering.

Allowed serial cases:

- `t.Setenv`, `os.Setenv`, `os.Clearenv`, or environment inheritance behavior
- `os.Chdir`
- process signal handlers
- fixed ports, named sockets, global loggers, global registries
- tests that prove serialization, lock contention, or ordering against one
  shared resource

Serial tests require a reason at the call site:

```go
func TestSignalHandler(t *testing.T) {
	// witness:waiver test/parallel/default -- installs process-wide signal.Notify handler.
}
```

Silence means the author forgot. It does not mean the test is exempt.

## Synchronization

`id: test/sync/no-sleep`

`level: must`

`enforcement: lint`

Tests must not use `time.Sleep` for synchronization.

Sleeping is a guess about scheduler timing. Guesses pass locally and fail on
loaded runners. Synchronize on facts: channel close, `sync.WaitGroup`,
`errgroup`, context cancellation, file appearance with bounded polling, or a
mock clock.

Wrong:

```go
go worker()
time.Sleep(100 * time.Millisecond)
if !done {
	t.Fatal("worker did not finish")
}
```

Right:

```go
done := make(chan struct{})
go func() {
	defer close(done)
	worker()
}()

select {
case <-done:
case <-time.After(10 * time.Second):
	t.Fatal("worker did not finish")
}
```

Timeouts are allowed as deadlock backstops. They are not allowed as proof that
work probably happened.

Timeouts should be generous enough to prove only "this is wedged", not
performance. Prefer a package-local timeout constant when several tests need the
same backstop. A timeout assertion must not be the only evidence that work
completed.

## Determinism

`id: test/determinism`

`level: must`

`enforcement: review`

Tests must control time, randomness, paths, environment, and process output.

Production code that needs current time, random values, generated IDs, or
external commands should accept those dependencies at the boundary. Tests supply
fixed values. Production supplies the real implementation.

Required defaults:

- use fixed timestamps
- assert stored timestamps as raw nanoseconds when the contract is persistence;
  assert formatted timestamps only when the contract is display
- use seeded random sources
- use deterministic IDs
- sort map keys before comparing serialized output
- avoid assertions on wall-clock duration outside benchmarks
- inject clocks or command runners at package boundaries; if third-party code
  calls `time.Now()` directly, wrap it before testing deterministic behavior

## Table Shape

`id: test/table-shape`

`level: must`

`enforcement: review`

When cases share one execution shape, use a table test.

The table is the pressure surface.

A table test is not a list of examples. It is an extreme-input proof set
designed to break the function. Happy-path sampling is not enough. The table
must pressure both sides of every expected boundary and the exact places the
implementation is most likely to lie.

The loop is just the executor. The table is the specification. Case names must
name the boundary, failure mode, or invariant being attacked, not vague labels
like `case 1`.

Required fields:

- `name`
- input or setup function
- `want*` result fields
- `wantErr`, `wantViolation`, or other `want*` contract fields when applicable
- why this case exists when the reason is not obvious from the name

Required adversarial shape:

- at least 10 expected-valid cases for non-trivial policy/data functions
- at least 10 negative/rejection cases for non-trivial policy/data functions
- at least 20 boundary cases distributed across both sides of the function's
  meaningful spectrum
- exact-boundary cases must include the value at the boundary, one below, and
  one above
- malformed input cases must include empty, truncated, oversized, unknown,
  duplicated, reordered, and type-wrong variants where relevant
- enum/domain tests must include every valid enum value plus unknown/future
  values
- parser tests must include hostile input, not just polite malformed input
- classifier/scoring tests must include tie, threshold, saturation, and
  precedence cases

For small functions with a smaller complete input space, exhaust the whole
space. The counts are floors for serious functions, not bureaucracy for trivial
helpers.

If a table has only two or three cases, it is probably not proving enough unless
the function is mathematically tiny.

Pattern:

```go
cases := []struct {
	name string
	in   Input
	want Result
}{
	{name: "valid floor timeout is accepted", in: Input{Timeout: floor}, want: Result{Timeout: floor}},
	{name: "zero timeout is rejected", in: Input{Timeout: 0}, want: Result{Err: ErrInvalidTimeout}},
	{name: "one below floor is rejected", in: Input{Timeout: floor - 1}, want: Result{Err: ErrInvalidTimeout}},
	{name: "one above floor is accepted", in: Input{Timeout: floor + 1}, want: Result{Timeout: floor + 1}},
}

for _, tc := range cases {
	t.Run(tc.name, func(t *testing.T) {
		t.Parallel()
		got := Resolve(tc.in)
		if got != tc.want {
			t.Fatalf("Resolve() = %v, want %v", got, tc.want)
		}
	})
}
```

Do not force a table when there is only one behavior and no meaningful case
matrix. The rule exists to make boundaries visible, not to add ceremony.

A weak table proves the author knew the function's intended happy path. A strong
table proves the function survives contact with bad inputs, edge cases, future
enum values, and maximally inconvenient state.

## Boundary Coverage

`id: test/boundaries`

`level: must`

`enforcement: review`

Policy/data packages need negative and boundary coverage.

For a function that validates, classifies, parses, grades, plans, or folds, a
happy-path-only test is incomplete.

Minimum coverage:

- valid input
- invalid input
- zero value
- exact threshold
- one below threshold
- one above threshold
- minimum representable value when applicable
- maximum representable value when applicable
- duplicate input when uniqueness matters
- missing input when required fields matter
- conflicting input when precedence matters
- unknown future enum when applicable
- malformed external input when parsing

This is why exact-at-threshold grade behavior belongs in a test: the contract is
different from one point below.

For serious validation, parsing, planning, scoring, folding, and classification
code, the expected default is:

- 10 cases that should pass
- 10 cases that should fail
- 20 boundary/hostile cases that probe both sides of the behavior spectrum

If those numbers feel excessive, the function is either trivial enough to
exhaust completely or important enough to deserve the proof.

`test/table-shape` governs the shape of the executor and case table.
`test/boundaries` governs what hostile dimensions must be represented inside
that table. Do not count the same three polite cases as both table shape and
boundary coverage.

A serious function is one that sits on a trust boundary, parser, classifier,
planner, scorer, ledger/fold path, artifact/finalization path, or doctrine
rule. Tiny field formatters and one-branch adapters can exhaust their complete
input space instead of meeting the 10/10/20 floor.

## Layer Triad Coverage

`id: test/layer-triad`

`level: must`

`enforcement: review`

Evidence and accounting changes must prove every layer they touch with a
positive, negative, and neutral case.

One triad at the highest layer is not enough. Each touched layer must have its
own local triad test. End-to-end and integration tests are valuable, but they do
not replace local producer, schema, writer, replay, fold, projection, verifier,
reporter, and CLI/process triads.

An evidence pipeline is not one function. A correct producer paired with a
blind verifier is a bug. A correct writer paired with a stale reporter is a
bug. A typed index that accepts a bad counter because a lower layer already
"should have" checked it is a bug.

Any change that affects captured output, fuzz evidence, retained facts,
artifacts, ledgers, manifests, terminal envelopes, run/fold accounting, or
verification must name the layers it crosses and add or update tests at each
crossed layer. Untouched downstream layers require an explicit reason in the
review or bug record.

Required layer map:

- producer/capture: execution code emits the right typed facts and refuses or
  marks bad source material
- core schema: records validate impossible, unknown, duplicate, missing, and
  zero-value combinations
- durable writer: files, indexes, shards, fsync/rename, and close failures keep
  accounting honest
- ledger writer/replay: envelopes seal, replay, reject tampering, and preserve
  final-state precedence
- fold/run accounting: planned, emitted, skipped, failed, aborted, and degraded
  facts stay balanced
- artifact/manifest projection: every durable evidence file is projected once,
  and scratch/raw/duplicate files are rejected or explicitly excluded
- hostile self-consistency scan: an independent walk over the finalized
  bundle compares the typed projection, the manifest, and the on-disk tree
  and refuses to certify when any two disagree
- verifier: the independent disk/manifest/ledger check catches forged,
  missing, duplicated, truncated, and extra evidence
- reporter/human output: typed evidence is rendered once and generic fallback
  output does not repeat or hide it
- CLI/process boundary: cancellation, close errors, setup/install state, and
  command-output capture do not silently downgrade evidence

The hostile self-consistency scan is a separate layer from verifier even when
they share code. Verifier proves the bundle is internally consistent against
its own claims. The hostile scan proves the producer/writer/projection chain
did not record presence the durable tree cannot sustain. A bundle that passes
verifier but fails the hostile scan has a producer/writer bug.

Required triad per layer:

- positive: the intended evidence path succeeds and the exact counters, files,
  hashes, states, and terminal facts match
- negative: a hostile or impossible input fails loudly with the strongest stable
  error contract available
- neutral: absent, empty, zero, skipped, disabled, duplicate-no-op, or
  below-threshold input does not create fake evidence, noisy artifacts, or
  misleading counters

Neutral is mandatory. Many evidence-pipeline bugs are not "bad input accepted";
they are "nothing happened but the ledger said something did" or "a tiny no-op
produced three durable files." A neutral test must assert both sides: the
expected absence of durable noise and the preservation of any required
zero/empty sealed fact.

The triad must be hostile, not polite. Examples:

- duplicate refs with identical path/hash/bytes and conflicting path/hash/bytes
- all dropped evidence, partially dropped evidence, and no reported evidence
- missing trailing newline, truncated JSON, oversized line, and empty stream
- forged index that matches disk size but lies about child hashes or counts
- successful seal, seal failure before rename, and seal failure after temp file
  creation
- completed run, aborted run, and multiple terminal envelopes where last wins

Structural tests may enforce that every evidence family supplies the layer
triad, but they do not replace the behavioral tests. The behavioral tests must
execute the real producer/writer/verifier path whenever that path is cheap
enough to run inside the package test.

Every package that owns one of the required layer responsibilities must
declare its local triad tests as top-level test functions whose names contain
`LayerTriad`. The doctrine ratchet locates the contract by function name
across the package's consolidated `_test.go`, so an auditor can grep for
`LayerTriad` to find the layer's positive/negative/neutral coverage without
threading through file boundaries.

## Package Sweep Exit Gate

`id: test/package-sweep-exit`

`level: must`

`enforcement: review`

A package sweep is not done because a few new tests were added. It is done when
the package's trust surfaces have explicit coverage and the old weak contracts
are retired.

Exit checklist:

- all new or touched rejection paths have typed sentinels or typed errors
- substring error assertions are gone or carry a waiver naming the upstream gap
- serious validators, parsers, classifiers, planners, folders, writers, and
  verifiers have hostile tables or exhaustive matrices
- evidence/accounting paths have local layer triads
- structural invariants have source/AST ratchets when runtime tests cannot
  cheaply prove them
- old compatibility tests have been upgraded or deleted
- no new dead wrappers, duplicate truths, or test-only shims were introduced
- focused package tests pass
- relevant lint (the project's doctrine linter, `go vet`, `staticcheck`, and
  any package-owned tool) is clean or has an explicit known baseline

For a large package, the sweep may land in slices. Each slice must say which
surface it closed and which surface remains. Do not declare the package done
while known hostile surfaces are only "documented" and not ratcheted.

## Error Assertions

`id: test/errors`

`level: must`

`enforcement: review`

Assert the strongest stable error contract available.

Order of preference:

1. `errors.Is` for sentinels
2. `errors.As` / `errors.AsType` for typed errors
3. category helpers like `os.IsNotExist`
4. `err != nil` only when platform differences make stronger checks brittle

Weak:

```go
if err == nil {
	t.Fatal("expected error")
}
```

Strong:

```go
if !errors.Is(err, ErrHashMismatch) {
	t.Fatalf("Replay() error = %v, want %v", err, ErrHashMismatch)
}
```

Do not compare error strings. Strings are diagnostics, not contracts.

### Two-Tier Assertions

When a diagnostic string still matters to an operator, the assertion is
two-tiered:

1. assert the typed class with `errors.Is` or `errors.As` — this is the
   load-bearing rejection proof
2. then assert the diagnostic detail only for the user-visible payload —
   this is the operator-facing message check, not the contract

The diagnostic check must never be the only rejection proof. A test that
matches only on the diagnostic string passes when the error class regresses
to a different typed error that happens to render the same prose.

If an upstream library exposes only strings, wrap the error at the boundary
with a typed or sentinel error and assert that wrapper. If no wrapper exists
yet, a string assertion needs a waiver that names the upstream gap and the
migration target.

## Structural Equality

`id: test/structural-equality`

`level: must`

`enforcement: lint`

Do not use `reflect.DeepEqual` in tests.

`reflect.DeepEqual` hides type-specific diffs, treats nil and empty slices in a
way that often conflicts with wire contracts, and encourages whole-record
comparison without explaining which field is load-bearing.

Preferred comparison order:

1. direct `==` for comparable values
2. `slices.Equal` / `maps.Equal` for comparable collections
3. `slices.EqualFunc` / `maps.EqualFunc` with domain comparators
4. explicit field-by-field comparison when each field is a contract
5. canonical JSON byte equality only when the JSON projection itself is the
   contract
6. `cmp.Diff` only in tests where a rich structural diff is worth the
   dependency and the comparison options are explicit

Wrong:

```go
if !reflect.DeepEqual(got, want) {
	t.Fatalf("record = %v, want %v", got, want)
}
```

Better:

```go
if got.Kind != want.Kind || got.Path != want.Path || got.SHA256 != want.SHA256 {
	t.Fatalf("artifact = %+v, want %+v", got, want)
}
```

## Helpers

`id: test/helpers`

`level: must`

`enforcement: lint`

Assertion frameworks and `assert*` helpers are banned.

Tests must show the fact being checked in plain Go with `got` and `want`
language. A failing test should tell the reader exactly what was observed and
what contract was expected without hiding the comparison behind a generic
assertion API.

Table tests must use `got` and `want` vocabulary. Do not name table fields
`expected`, `actual`, `expectedResult`, or `actualOutput`. The table describes
the contract with `want*` fields; the test body computes `got`.

Subtest names must name the behavior or hostile condition. Names like `case`,
`valid`, `invalid`, `success`, `failure`, `ok`, `true`, and `false` are too
generic.

Forbidden:

- `github.com/stretchr/testify/assert`
- `github.com/stretchr/testify/require`
- local helpers named `assert*`
- local helpers named `check*`, `expect*`, or `require*` when they hide the
  observed value or expected contract
- table fields named `expected*` or `actual*`
- generic subtest names that hide the boundary being exercised
- failure messages like `expected X, got Y`
- failure messages that hide the observed value

Allowed:

- direct `if got != want { t.Fatalf(...) }`
- small `require*` setup helpers for fixture construction or repeated
  fail-fast preconditions
- domain-specific helpers that preserve `got`/`want` in the failure text
- helper functions that accept `*testing.T` and call `t.Helper()` before any
  failure path

Helpers should make failures sharper, not hide complexity. If a helper needs
branch-heavy logic, split it or return structured data and check it in the
test.

Pattern:

```go
got := Verify(record)
want := Verification{Status: VerificationVerified}
if got != want {
	t.Fatalf("Verify() = %v, want %v", got, want)
}
```

Helper pattern:

```go
func requireVerified(t *testing.T, got Verification, want Verification) {
	t.Helper()
	if got != want {
		t.Fatalf("verification = %v, want %v", got, want)
	}
}
```

## Mutable Fixtures

`id: test/fixtures/no-shared-mutable`

`level: must`

`enforcement: lint`

Parallel tests must not share mutable fixtures.

Package-level maps, slices, pointers, and buffers are allowed only as immutable
templates. Each test case must clone or construct its own mutable value.

Preferred table shape:

```go
cases := []struct {
	name  string
	setup func() *Config
}{
	{
		name: "default tags",
		setup: func() *Config {
			return &Config{Tags: []string{"a"}}
		},
	},
}
```

## Non-Vacuous Fixtures

`id: test/fixtures/non-vacuous`

`level: must`

`enforcement: review`

Fixtures must force the behavior under test to execute.

A shared fixture that makes one table row or one projected writer no-op can
produce a green test that proves nothing. Every fixture-driven test must have a
minimum-output, minimum-effect, or explicit branch assertion that would fail if
the target silently skipped its work.

Required when a test calls multiple conditional implementations through one
harness:

- each row declares the minimum facts, files, records, or state transitions it
  must produce
- pass and fail branches use different fixtures when the production code writes
  different artifacts
- a zero-output result is accepted only when the case name says the no-op is the
  contract

## Production Path Proof

`id: test/production-path`

`level: must`

`enforcement: review`

A test must be honest about which path it exercises.

Direct helper tests are useful, but they are not integration proof. Fake
commanders, pre-baked refs, pre-written sidecar files, synthetic ledger lines,
and in-memory publishers must be named as such in the test or review note. If
the commit message claims the real durable path is covered, the test must drive
the real producer, writer, commit, replay, or verifier path.

Common false coverage patterns:

- a worker test pre-writes the final sidecar file even though production writes
  a temp file and commits it later
- a fake commander pre-populates `StdoutRef` while the claimed contract is
  runwriter post-fill
- a report test string-matches Markdown while the machine JSON projection is
  the stable surface
- a verifier test names a manifest drift but skips the bundle manifest writer
  that normally creates the input

When only a lower-level unit is practical, say so and add the next integration
surface as a separate slice. The distinction matters because pipeline bugs often
live between two correct local components.

## Evidence Path Consistency

`id: test/evidence-path-consistency`

`level: must`

`enforcement: review`

A test that asserts a captured evidence file must prove the three-way
invariant: the typed reference names the file, the durable manifest seals
the file, and the file exists on disk at the asserted path.

Three independent layers can disagree about a single evidence file:

- the typed reference that the producer records in the run projection
- the final manifest that the seal/finalize path emits
- the bytes on disk under the bundle root

Two of three agreeing is not enough. A test that only checks the typed
reference passes when the manifest is missing the entry. A test that only
checks disk presence passes when the projection never recorded it. The
hostile self-consistency scan (see `test/layer-triad`) catches divergence at
run time, but local tests must not let the run get that far.

Required for every test that touches captured evidence:

- assert the typed reference names the expected path, hash, and byte count
- assert the manifest contains the same path with matching hash and byte
  count
- assert the file exists on disk at the manifest-asserted path with the
  manifest-asserted bytes
- assert the manifest entry was produced by the projection writer being
  tested, not by a pre-baked fixture

Triad reminder: this rule is in addition to `test/layer-triad`. The triad
governs per-layer positive/negative/neutral coverage. This rule governs
cross-layer consistency for a single piece of evidence.

Common failure classes this rule catches:

- the run projection records a path that the writer never wrote
- the writer seals a file but the manifest projection skips it
- the manifest claims bytes the disk file no longer has after a cleanup pass
- profile or benchmark capture records the reference optimistically before
  the subprocess writes a non-empty file

Helper pattern:

```go
func requireEvidenceConsistent(t *testing.T, ref EvidenceRef, m Manifest, bundleRoot string) {
	t.Helper()
	entry, ok := m.Lookup(ref.Path)
	if !ok {
		t.Fatalf("manifest missing %s; ref says present", ref.Path)
	}
	if entry.SHA256 != ref.SHA256 || entry.Bytes != ref.Bytes {
		t.Fatalf("%s manifest=%s/%d ref=%s/%d", ref.Path, entry.SHA256, entry.Bytes, ref.SHA256, ref.Bytes)
	}
	info, err := os.Stat(filepath.Join(bundleRoot, ref.Path))
	if err != nil {
		t.Fatalf("stat %s: %v", ref.Path, err)
	}
	if info.Size() != entry.Bytes {
		t.Fatalf("%s disk=%d manifest=%d", ref.Path, info.Size(), entry.Bytes)
	}
}
```

`EvidenceRef` and `Manifest` are placeholders for whatever typed reference
and manifest types the package owns. The point is the three-way check, not
the helper signature.

## Goroutine Ownership

`id: test/goroutines/owned`

`level: must`

`enforcement: review`

Every goroutine started by a test must have an owner and an exit proof.

Required proof:

- a cancel path
- a wait path
- a timeout backstop
- no assertion on global `runtime.NumGoroutine`

`runtime.NumGoroutine` is process-global. It changes with the Go runtime, test
runner, coverage, race detector, and unrelated packages. It does not prove that
the goroutine this test started exited.

Pattern:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

done := make(chan error, 1)
go func() {
	done <- run(ctx)
}()

cancel()
select {
case err := <-done:
	if err != nil {
		t.Fatal(err)
	}
case <-time.After(10 * time.Second):
	t.Fatal("worker did not exit")
}
```

## Structural Invariant Tests

`id: test/structural-invariant`

`level: must`

`enforcement: review`

Source-scanning, reflection-shape, discovery-fence, and allowlist-ratchet tests
are valid when the contract is compiler-visible structure that runtime behavior
cannot prove.

Use these tests for architecture boundaries:

- a finalization stage must not accept a mutable pointer
- a projection carrier must have exactly the expected fields
- every writer helper must be classified
- every raw write must be projected or explicitly scratch
- an allowlist must shrink or stay fixed

Required shape:

- the test states the structural invariant in its name
- the source scan is narrow and deterministic
- failure output names the missing or forbidden symbol
- allowlists require a reason string and a ratcheted maximum count
- synthetic-source fixtures share the same matcher as the live repository scan
  when the matcher is non-trivial
- the scan proves structure, not prose; avoid broad string searches when an AST
  shape can be matched
- runtime tests still prove behavior when behavior can be executed cheaply

Structural tests are not substitutes for red/green behavioral proof. They lock
architecture after the behavior is understood.

These scans are test-only. Production evidence capture stays streaming and must
not build a world model just to enforce a doctrine rule.

## Data-Flow Struct Inventories

`id: test/data-flow-inventory`

`level: must`

`enforcement: review`

Every package that owns a trust-boundary surface must maintain a data-flow
struct inventory and treat it as a wiring ratchet.

Trust-boundary surfaces include protocol parsing and emission, evidence
capture and projection, durable writing and replay, run/fold accounting,
manifest generation, verification, and doctrine enforcement. Packages that
do not own such a surface — small leaf helpers, pure-format utilities, thin
glue — are not required to maintain an inventory, but adding one is never
wrong.

Every production struct in an eligible package must appear in the inventory
and be classified into an intentional role: protocol fact, sealed projection,
internal flow carrier, test-only seam, capability wrapper, or another
package-local category with the same precision.

The inventory does not prove behavior by itself. It proves that the compiler
can see the shape and that reviewers cannot accidentally miss a new data
carrier. Behavior still needs hostile tests, `Validate()` gates, typed
sentinels, layer-triad coverage, and end-to-end evidence checks where
appropriate.

Required:

- newly added production structs must be added to the package inventory in the
  same slice
- the classification must describe the struct's role in the execution pipeline,
  not just its file location
- internal DTOs, overlay keys, outcome maps, and typed handoff structs are not
  exempt; they should usually be classified as internal flow
- wire/protocol structs must be distinguished from internal flow structs
- deleted structs must be removed from the inventory in the same slice
- failure output must name the missing or unclassified struct

Forbidden:

- allowing a new production struct to compile without an inventory entry
- hiding a data carrier behind `map[string]any`, raw JSON, or an untyped helper
  to avoid inventory classification
- treating the inventory as behavioral proof; it is a wiring proof only

Example: a temporary-looking finalize-time identity struct that routes one
event's outcome to a specific transform is still a production struct. It must
be classified, because it decides which event receives the transform. The right
inventory role is internal flow; the behavior is then proved separately by
hostile tests showing same-key siblings do not receive each other's transforms.

## Repeat Policy

`id: test/repeat-policy`

`level: must`

`enforcement: review`

`RepeatCount` applies to Go tests only.

Tools, benchmarks, and fuzz targets are evidence phases, not repeat-amplified
test loops. Running them multiple times by default adds noise, distorts timing,
and creates duplicate sidecars without increasing confidence in the same way
test repetition does.

Required phase policy:

- tests may repeat according to `RepeatCount`
- tools run once per run unless a tool-specific contract says otherwise
- benchmarks run once per configured benchmark pass
- fuzz targets run once per configured fuzz budget
- repeated phases must be explicit in the run record and final artifacts

## Budget Divergence

`id: test/budget-divergence`

`level: must`

`enforcement: review`

The ratio of effective to configured budget is a contract, not a side
effect.

Run profiles declare configured budgets for tests, benchmarks, fuzz,
per-package timeouts, and aggregate timeouts. The orchestrator computes
effective budgets from configured budgets, parallelism, repeat counts, and
the unit count of each phase. A large divergence ratio means one of:

- the profile under-declares the work the run actually does
- the orchestrator silently amplifies a small budget into hours of execution
- the unit count grew without the profile being re-tuned

Any of those should be visible, not invisible.

Required:

- profile changes that move a configured budget must state the expected
  effective range in the commit
- a run with `effective / configured > 10×` for any budget must surface that
  ratio in the human-facing report and in the machine record
- a run with `effective / configured > 100×` for any budget is an
  infrastructure violation; either the profile is wrong or the amplification
  is wrong, and certification must not bury the discrepancy
- changes to amplification factors (parallelism, repeat count, unit-count
  multipliers) must update tests that assert the resulting effective budgets

Forbidden:

- treating effective budgets as derived telemetry without a contract
- profiles that configure tiny budgets relying on amplification to make the
  run feasible; configure the real intended budget
- silencing divergence by raising the configured budget post-hoc instead of
  fixing the source of amplification

Budget divergence is also a diagnostic signal. Stable allocation counts with
noisy wall-clock numbers in benchmarks indicate contention, not flakiness;
large budget divergence indicates orchestration drift, not flexibility.

## Benchmarks

`id: test/benchmarks`

`level: must`

`enforcement: review`

Benchmarks must measure one thing and report allocations.

Required shape:

- setup before `b.ResetTimer()`
- `b.Loop()` may replace an explicit `b.N` loop and timer reset when the Go
  version supports it
- `b.ReportAllocs()`
- no logging in the timed loop
- no filesystem/network setup in the timed loop unless that is the benchmark
- stable input size
- benchmarks run serially and after tools and tests in the run orchestration
  unless the benchmark explicitly measures parallel behavior
- benchmark stdout, stderr, profiles, and retained binaries are evidence and
  must land on disk when captured
- captured benchmark evidence must be covered by the final manifest
- CPU and memory profiles, when requested by a profile, must be retained as
  evidence with durable refs and manifest coverage

Pattern:

```go
func BenchmarkEncodeEnvelope(b *testing.B) {
	record := fixtureEnvelope()
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, _ = Encode(record)
	}
}
```

## Fuzz Tests

`id: test/fuzz-boundary`

`level: should`

`enforcement: review`

Fuzz tests belong at trust boundaries.

Good fuzz targets:

- JSON/JSONL parsing
- CLI argument parsing
- ledger replay
- profile parsing
- enum unmarshalling
- externally supplied sidecar content

Poor fuzz targets:

- pure functions over already-validated structs
- code whose valid input space is tiny and better covered by a table

Fuzz phase policy:

- fuzz targets run serially and after tools/tests/benchmarks unless the fuzz
  target explicitly proves parallel behavior
- stdout, stderr, crashers, and promoted corpus snapshots are evidence and must
  land on disk when captured
- fuzz cache, minimization scratch, and junk corpus churn must not enter the
  final bundle unless promoted as explicit evidence
- timeout or minimization quirks must be classified as harness behavior, not
  target-code failure, when no target failure was produced

Every fuzz callback must own an oracle that can reject a semantically wrong
result. "Did not panic" is a runtime safety property, not sufficient evidence
for a parser, verifier, writer, classifier, or fold. Mutated input must reach
the oracle inside the fuzz callback; a sibling seed table does not substitute
for it.

Required shapes include at least one independent invariant appropriate to the
boundary:

- accepted parse -> `Validate()` succeeds -> marshal/parse round trip preserves
  the typed value
- rejected parse -> stable typed error identity and a zero/non-mutated receiver
- signed input -> accepted if and only if the independently pinned fields and
  signature match
- writer -> bounded output, canonical second write, and decode/validate parity
- fold/replay -> arithmetic, sequence, chain-link, or final-state invariants
- differential parser -> two independent representations or implementations
  agree

```go
f.Fuzz(func(t *testing.T, data []byte) {
	got, err := Parse(data)
	if err != nil {
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Parse() error = %v, want %v", err, ErrInvalidInput)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Parse() accepted invalid value: %v", err)
	}
	encoded, err := got.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}
	roundTrip, err := Parse(encoded)
	if err != nil || roundTrip != got {
		t.Fatalf("round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
})
```

Delete fuzz targets for tiny closed input spaces already exhausted by hostile
tables. They consume continuous-fuzz budget without creating new evidence.

## Streaming And Ledger Proofs

`id: test/ledger-chain`

`level: must`

`enforcement: review`

Ledger tests must prove the real chain when the behavior concerns evidence.

If a test claims writer/replay/fold correctness, it should write through the
real writer, replay through the real verifier, and assert the chain status and
fold result. Mocking the ledger in those tests proves the mock.

Required assertions:

- sequence monotonicity when sequence matters
- previous-hash linkage
- final hash or verification status
- corruption/truncation failure mode for negative cases

## Protocol Types

`id: protocol/typed-boundary`

`level: must`

`enforcement: lint`

Protocol payloads must be typed structs and enums.

Forbidden in protocol-bearing code:

- `any`
- `interface{}`
- `map[...]any`
- generic instantiations that smuggle `any`
- silent enum switch defaults
- string-backed domain types where an iota enum is correct

Allowed:

- `any` as a generic type parameter constraint
- `json.RawMessage` at explicit wire/replay boundaries only
- `interface` when it is a narrow behavior interface, not a payload bag

## Waivers

`id: test/waivers`

`level: must`

`enforcement: review`

A waiver must explain why the rule is wrong for this specific case.

Good:

```go
// witness:waiver test/parallel/default -- verifies process-wide environment inheritance.
```

Good:

```go
// witness:waiver test/isolation/tempdir -- verifies global lock contention on one path.
```

Bad:

```go
// flaky in CI
```

Bad:

```go
// TODO
```

Waivers are not permission slips. Where the typed helper is available, tests do
not use serial waiver comments at all: they call `testserial.Declare` with a
Primitive-owned `core.TestIsolationDeclaration`. That typed call is the
compiler-visible reason the test cannot call `t.Parallel`, and invalid or zero
declarations fail at execution. The doctrine linter verifies the exact imported
package, function, hazard identifier, and scope identifier contract.

A typed declaration beats a comment for the reason this whole protocol exists:
a comment cannot be validated, cannot fail at execution, and cannot break the
build when the hazard it names stops being true.

Canonical grammar:

```text
// witness:waiver <rule-id> -- <specific reason>
```

The reason must name why this case is structurally different, not why fixing it
is inconvenient. Legacy local comments such as `serial:` are rejected. The
comment grammar remains available only where the typed helper has not yet been
installed; once a repository has it, the ratchet rejects both forms.

## Automation Map

Each repository's canonical gate installs and runs its pinned doctrine-linter
version once a production package exists. Each row names the rule or sub-rule
and the lint surface that owns enforcement so a reader can locate the
implementation by grepping the implementer column.

The gate discovers the landed production packages from the module and runs the
pinned analyzers over that exact set. An empty set is reported as
`NOT_APPLICABLE`; with landed packages, an analyzer result is package evidence
and no design-phase exception exists.

| rule or sub-rule | implemented by |
| :--------------- | :------------- |
| `test/isolation/tempdir` | lint: filesystem-mutation analyzer |
| `test/sync/no-sleep` | lint: `time.Sleep` rejector in tests |
| `test/parallel/default` | lint: missing-`t.Parallel()` analyzer |
| `protocol/typed-boundary` | lint: protocol-bag (`any`/`map[...]any`) rejector |
| `test/helpers` (assertion-framework imports) | lint: import allowlist for `testing` packages |
| `test/helpers` (assert-style local helpers) | lint: function-name analyzer for `assert*`/`expect*` |
| `test/helpers` (got/want vocabulary) | lint: table-field and failure-message wording analyzer |
| `test/helpers` (generic subtest names) | lint: subtest-name vocabulary analyzer |
| `test/benchmarks` | lint: benchmark shape and `b.ReportAllocs` requirement |
| `test/structural-equality` | lint: `reflect.DeepEqual` rejector in tests |
| `test/fixtures/no-shared-mutable` | lint: mutable package-level test variable rejector |
| `TestMain` must call `m.Run()` or a recognized goleak wrapper | lint: `TestMain` body analyzer |
| `runtime.NumGoroutine` and `time.Now` in tests | lint: nondeterminism-source rejector |
| loud enum switch defaults | lint: switch-default analyzer for typed enums |
| streaming-critical no whole-stream reads | lint: streaming-budget analyzer |

Next good automation targets. Each is currently `review`-enforced and should
graduate to lint when its false-positive story is understood.

| rule or sub-rule | candidate lint surface |
| :--------------- | :--------------------- |
| `test/table-shape` (named subtests) | table-test-shape analyzer |
| `test/structural-invariant` (allowlist ratchets) | allowlist count-monotonicity analyzer |
| `test/fixtures/non-vacuous` | minimum-output checker for common harnesses |
| `test/fuzz-boundary` (panic-free assertion) | fuzz-target body analyzer |
| `test/package-sweep-exit` (sweep metadata) | package-sweep-marker scanner |
| `test/production-path` (fake commanders / pre-baked refs) | production-path proof-marker scanner |
| `test/compiler-driven` (stale legacy-compat branches after clean-break) | dead-branch ratchet against the new typed contract |
| `test/evidence-path-consistency` | three-way-invariant analyzer for evidence-ref tests |
| `test/budget-divergence` | run-record analyzer flagging ratio thresholds |

Do not automate a rule until the false-positive story is understood. A noisy
doctrine rule creates waivers, and waivers rot.
