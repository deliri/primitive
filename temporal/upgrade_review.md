# Temporal upgrade review

The later [bug-report follow-up](../hostfacts/review_followup.md) contains the current changes,
verification and profiled measurements. The evidence below describes the earlier
upgrade checkpoint.

Temporal is ready for user review. This is author-produced local evidence, not
independent acceptance. No commit, version bump or release was performed for
this upgrade. Hostfacts is part of the current review batch; Process follows.

## Production findings

Two defect classes were demonstrated against the original production source:

1. JSON decoding could lose `core.ErrJSONContract` after successful lexical
   decoding but failed canonical, range or domain validation. All five JSON
   decoding boundaries now retain JSON and Temporal error identities, preserve
   native parse causes where applicable, and leave receivers unchanged on error.
2. `IntervalRequest.Validate` could accept observations whose corrected start
   wall plus elapsed duration overflowed, although construction refused them.
   Validation now uses the complete construction rule. Bounds construction also
   avoids repeated validation walks. Tests cover the adjacent accepted boundary,
   corrected wall times, reversal and maximum representable spans.

The original hostile run recorded 14 failing leaf cases plus two failed parent
events. Those are evidence of the two defect classes, not 16 independent bugs.

Profile-informed changes replace temporary canonical strings with Go's
`strconv.AppendInt`, `jsontext.AppendQuote` and `Time.AppendFormat`, using fixed
arrays. A validated zero wait returns without acquiring a timer. Positive waits,
tickers, cancellation, parsing and quoting still use Go's standard library.
There is no new exported API, dependency, scheduler or production global state.
Duration text retains Go's syntax and input-length cost; this report does not
claim constant-time parsing of arbitrary text.

## Tests and protocol

The complete 2,214-line project testing protocol was read before test changes.
Direct tables replace weak grouped examples for timeout/deadline propagation,
waits, tickers, observation arithmetic, interval bounds, numeric persistence and
aggregate parsing. Cases attack cancellation causes, earlier/equal/later parent
deadlines, corrupt private values, receiver preservation, exact output and the
absence of effects on refusal. Go's `testing/synctest` supplies deterministic time.

The aggregate parser has a direct 40-case matrix. Numeric encoding tables now
decode and compare exact values. The assertion-hiding `requireNanoseconds`
helper and duplicated standalone aggregate maximum JSON example were retired.
The struct inventory derives actual type names from its typed fields; it no
longer relies on a duplicated list of names. Compile witnesses cover UTC APIs.

Fuzz oracles check semantic acceptance as well as rejection: RFC3339 and compact
UTC compare independent grammar/stdlib parsing and representability rules;
aggregate arithmetic compares bounded `math/big` calculations. JSON fuzzing
checks canonical output, domain/error identities and receiver preservation.

Eleven deliberate source mutations were rejected by semantic test failures:
signed canonical spelling, quoted escaping, native numeric causes, receiver
preservation, derived interval end, zero-wait cancellation, aggregate add carry,
aggregate multiply carry, RFC3339 strict syntax, false RFC3339 refusal and
inventory binding. None was counted merely because a mutant failed to build.

## Verification

On Go 1.27.1, Darwin/arm64:

- Final package tests: 744 passing Go test events, zero failures or skips;
  95.2% statement coverage. The original baseline had 601 passing events and
  90.8% coverage. Event counts include parent tests and fuzz seeds.
- Race run: the same 744 passing events, with a recorded shuffle seed.
- Ten fuzz targets: 30 seconds each, serial execution with one worker;
  6,519,244 total executions, no failures and no targets left unrun.
- Package-scoped vet, staticcheck, errcheck, Witness lint and production
  `gocyclo -over 10` passed. Go fix was applied earlier in the candidate.
- Linux/amd64 and Windows/amd64 test binaries compiled successfully. Runtime
  execution was on Darwin only.

Witness retains one narrow test waiver for the exact `context.Context.Value`
signature used by a hostile parent-context probe. The first lint attempt also
caught a direct sentinel comparison, which was changed to `errors.Is`.
The original package checkpoint preceded the full-module gates. The subsequent
Hostfacts review batch passed the requested analyzers across all 61 packages,
uncached module tests and two shuffled race passes. See
[the combined gate report](../hostfacts/upgrade_review.md) for exact scope,
retained failures and three credentialed GCS skips. This does not claim the
entire canonical release script or live-provider acceptance ran.

## Benchmarks and evidence

The [benchmark report](upgrade_benchmarks.md) contains all 14 comparisons and
their limitations. Each measured workload requests 30 seconds with CPU and
memory profiles plus the matching test binary. The three fastest operations
use identical 64-call batches in both compared phases. The audit verified that
paired benchmark function bodies are identical.

Numeric JSON decoding, zero waits and compact UTC parsing now allocate zero
times per call in the measured workloads. Quoted JSON decoding falls from four
or five allocations to one. These are single samples per workload per phase;
unchanged controls moved substantially, so no stable latency improvement is
claimed. The final compact UTC change has a focused refreshed measurement;
other optimized rows retain their exact earlier candidate source bindings.

[Machine-readable evidence](upgrade_evidence.json) records commands, toolchain,
environment, source manifests, test events, all benchmark samples, fuzz results,
mutation outcomes and artifact digests. The audit verified 66 execution records
and 443 retained files with no problems. Raw logs, source snapshots, profiles
and binaries remain ignored under `testdata/test-upgrade-20260907/`.

One capture attempt failed before running complexity analysis because concurrent
snapshot writers could expose a partially written blob. The harness was changed
to publish snapshots atomically, the failure record was retained, and the check
was rerun successfully. All final snapshot and artifact hashes were verified.
Intentional source changes during Go fix are recorded separately; failed and
superseded attempts remain in the evidence.

## Full-module review refresh

NilAway found a nullable post-loop benchmark result in `BenchmarkWithTimeoutCancel`.
An explicit nil guard now precedes its final `Err` check. The timed loop and
production behavior are unchanged. A focused 30-second profiled measurement on
that final source recorded 377.3 ns/op, 272 B/op and four allocations; the earlier
363.9 ns/op sample is retained as an earlier source phase, not overwritten.
Full-module tests reconfirmed 744 Temporal pass events and no failures/skips;
the two-pass race run recorded 1,488. The Hostfacts evidence manifest contains
the exact final source, refreshed benchmark binary/profiles and module gates.
The original Temporal evidence manifest remains the original checkpoint record.
