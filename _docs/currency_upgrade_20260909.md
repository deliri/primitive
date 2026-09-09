> User approved this reviewed slice for v2026.1.33: bump, commit, push, then ID.
> The measurements and awaiting-review wording below describe the preserved pre-release evidence.

# Currency upgrade — ready for review — 2026-09-09

Base: `v2026.1.32`, `2c1987775687a283a6ab5d940c4b60a00ece60ca`.
Work and retained raw evidence are on Furnace, Go 1.27.1 linux/amd64,
GOWORK=off and GOMAXPROCS=8. The complete 2,214-line local testing protocol
was read before test edits.
Candidate Go-source digest: `e707459172fc9322d76fc85fae9d651b52b52f895af69e026d672a95e54bc246`.
This is author-produced execution evidence awaiting user review, not an
independent acceptance receipt. Currency has not been bumped, committed or pushed.

## Production change

**One reproduced error-path defect:** the amount JSON decoder discarded
strconv.ParseInt's syntax and range failures. A JSON string one beyond either
int64 boundary lost both the Go range identity and Currency's shared numeric
overflow classification. The new regression failed in three named cases against
the original json.go bytes; its two canonical-form refusal controls passed.
The complete red source snapshot records that decimal conversion work was
already present; this is not represented as a completely untouched source tree.

The JSON decoder now preserves the native NumError and syntax/range identity.
An out-of-range value also carries ErrCurrencyOverflow and ErrNumericOverflow,
while retaining ErrCurrencyDecimal and ErrJSONContract. Noncanonical leading
zeros and plus signs still refuse without fabricating a native parse failure.
Fresh and populated receivers remain unchanged on every refusal.

Profiles also justified a narrow decimal implementation change:

- strconv.ParseUint replaces the local unsigned accumulator. Grammar, decimal
  exponent and signed-domain checks remain Currency-owned; native Go conversion
  errors stay reachable.
- strconv.AppendInt formats the signed integer, including MinInt64, into a
  fixed stack buffer. Currency inserts its decimal separator and bounded zero
  padding into another fixed buffer, then creates the result string once.
- The handwritten accumulator, signed-magnitude formatter and concatenating
  fixed-exponent helper were deleted.

Production behavior changes in decimal.go, json.go and errors.go.
contracts.go clarifies the shared decimal input/output bound; the Core overflow
identity comment now covers conversion as well as arithmetic. Public signatures, currency
tokens, JSON shape, accepted grammar and bounds are unchanged. There are no new
production structs, dependencies, pools, runtime state, providers or policy.
The fixed definition table remains intact despite its visible CPU cost; this
slice does not replace it with global mutable storage or a new lookup scheme.
Processing remains bounded by compiler-owned decimal/JSON limits and fixed buffers.

## Review findings addressed

The review found no additional reachable production bug. Its two suggestions
are addressed and its buffer observation is clarified:

1. The production unsigned converter now attaches overflow only to Go ErrRange.
   Syntax keeps ErrCurrencyDecimal and the native NumError, with no numeric
   overflow identity. A small private conversion function lets the hostile table
   exercise actual strconv parsing without weakening the public decimal grammar.
   The corrected table failed in three syntax cases when blanket overflow wrapping
   was restored, then passed with range-only classification. This is an internal
   boundary ratchet: current public grammar already excludes those syntax inputs.
2. overflowError returns Core's existing ErrCurrencyOverflow directly. It removes
   the misleading arithmetic-only diagnostic and its extra error construction;
   all typed parent identities remain intact. No error-text assertion was added.
3. DecimalMaximumBytes explicitly owns both input and canonical output bounds.
   The existing closed-domain extrema table checks the exact maximum and every
   round trip. A fifth fractional digit would still fit: changing point placement
   within a 19-digit magnitude does not add digits. The current admitted exponent
   set remains 0, 2, 3 and 4. No separate capacity constant or fallback was added.

An initial new test incorrectly expected ErrCurrencyDecimal on unsigned range
errors. That identity was never part of the Parse overflow contract. The initial
red and first green attempt retain this test error; the corrected red isolates
three actual internal syntax misclassifications. Final tests require exact
syntax/range identity exclusivity, native cause and input, zero refusal result,
and valid zero/MaxUint64 preservation.

The original review and manifest remain in pre-review under the raw evidence
root. The findings document is retained there verbatim. Earlier benchmark and
fuzz runs remain historical source-bound observations, not current-source proof.

## Public boundary inventory and hostile proof

| Surface | Proof |
| --- | --- |
| Code domain, String, FractionDigits, New | All 256 backing values at admission/projection; exact retained currency and signed extent; canonical definition matrix |
| ParseCode / Code.UnmarshalJSON | Closed-domain and independent standard JSON-token fuzz oracles; fresh/populated refusal preservation; below/at/above code JSON ceiling |
| New external numeric ingress | New FuzzAmountNominalArithmetic accepts raw code bytes and int64 operands; no modulo repair into the valid domain |
| Add / Subtract / Compare | Arbitrary-precision differential arithmetic, overflow zero results, mismatch-before-arithmetic precedence, exact ordering and local arithmetic LayerTriad |
| Parse | Independent regular-expression grammar and big.Rat numeric oracle; raw invalid code bytes now reach the decoder; byte, exponent, sign, padding and signed/unsigned ceilings |
| Decimal | Independent big.Rat.FloatString expectations at every int64-safe decimal digit-width transition, both signs, zero and extrema across all four exponent families |
| Amount.UnmarshalJSON / private minorUnitsJSON.UnmarshalJSON | Standard token-stream semantic fuzz, closed fields, duplicate/missing/unknown/type-wrong documents, native integer refusal table, unchanged receivers |
| Amount.MarshalJSON | Exact golden wire, real typed canonical fixtures, field order and accepted representation variants, local JSON projection LayerTriad including present zero |
| Compiler-visible structure | Four production structs retain classified roles; exact public/import/type-alias/map boundaries; six decoder/constructor doors map to five semantic fuzz owners |

The existing nominal schema LayerTriad remains. This package has no durable
writer, ledger, verifier, OS effect or independent producer/classifier evidence
pipeline; those layers and a synthetic 50-row handoff are not fabricated.

Architecture scans now read compiler-embedded source and parse synthetic inputs
in memory. The live and synthetic tests share their AST matcher. They perform
no direct runtime filesystem operations.

Removed or strengthened weak proof:

- An arithmetic rejection must return the exact zero amount, not merely an
  error beside an unchecked value.
- Two duplicate arbitrary-precision operand pairs and the four-row empty-token
  projection table were removed. Empty-token refusal remains covered through
  the real token boundary; invalid code projections are exhausted separately.
- Canonical valid JSON table fixtures now come from real typed Amount encoding.
  Deliberately malformed or alternate wire representations remain explicit.
- Amount JSON byte-limit cases now call Amount.UnmarshalJSON directly. Going
  through the outer Go decoder could discard padding before Currency saw it.
- The decoder inventory fails if a new external door lacks a named semantic
  fuzz owner.
- Benchmarks check exact outputs rather than discarding their final results.

## Red, green, mutation and tool accounting

Original race run: 413 passing test
events including parents, zero failures/skips, 93.4% coverage.
Final race run: **1188 passing events**, **35
top-level functions**, zero failures/skips, **94.6% statement coverage**.

Final scoped go vet, Staticcheck, Errcheck, Witness and production gocyclo <=10
pass. macOS arm64 and Windows amd64 test binaries compile; they were not run.
All final runs match the final package source digest exactly.
Full-module gates and consumer updates were not run.

Six deliberate semantic mutations were killed during the original slice: changed nominal units, removed
negative formatting, ignored currency mismatch, suppressed JSON refusal, added
an unfuzzed decoder, and dropped the new native unsigned-conversion cause.
One earlier JSON mutation accidentally commented out unrelated declarations and
failed to compile. It is retained and excluded from behavioral-kill accounting;
the corrected mutation compiled and failed the advertised refusal test.

The original production already passed the new nominal, arithmetic, formatting
and inventory ratchets. Those are stronger proof, not invented production bugs.
The JSON native-error defect has its separate recorded red state.

One Witness pass found two missing parent-level benchmark ReportAllocs calls
and a direct test error comparison. Both were corrected, with all attempts
retained. There was no failed-fuzz retry or favorable benchmark sample selection.
After the review changes, all eight benchmark workloads and all five fuzz
targets were rerun against the final source; historical runs are retained.

## Benchmarks and profiles

Each measured revision uses eight workloads, one pass and 30 seconds per case.
The table compares original production with the post-review candidate. The
pre-review candidate remains below as a separate historical observation:

```text
go test -run=^$ -bench=. -benchmem -benchtime=30s -count=1 -timeout=10m
  -cpuprofile=<phase>/cpu.pprof -memprofile=<phase>/mem.pprof
  -o <phase>/currency.test ./currency
```

The original production was captured before edits. Baseline and candidate have
identical fixtures, timed loops and result checks. Their benchmark files differ
only by two parent-level ReportAllocs calls added after the lint finding; every
timed child already called ReportAllocs. Removing exactly those two calls
reconstructs the baseline file byte-for-byte. This is recorded as an explicit
harness delta, not a byte-identical file claim.

| Workload | Before ns/op | Candidate ns/op | Observed change | B/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| `BenchmarkParseDecimal/minimum-four-digit` | 255.3 | 413.2 | +61.85% | 24 → 24 | 1 → 1 |
| `BenchmarkParseDecimal/short-two-digit` | 144 | 175.7 | +22.01% | 4 → 4 | 1 → 1 |
| `BenchmarkParseDecimal/overfull-fraction` | 184.6 | 298.2 | +61.54% | 56 → 56 | 2 → 2 |
| `BenchmarkFormatDecimal` | 216.7 | 189.9 | -12.37% | 72 → 24 | 3 → 1 |
| `BenchmarkAmountJSON/encode-maximum` | 1979 | 3364 | +69.98% | 232 → 232 | 10 → 10 |
| `BenchmarkAmountJSON/decode-maximum` | 5622 | 7104 | +26.36% | 1163 → 1163 | 22 → 22 |
| `BenchmarkCheckedAdd` | 43.89 | 42.26 | -3.71% | 0 → 0 | 0 → 0 |
| `BenchmarkParseCode` | 55.61 | 55.24 | -0.67% | 0 → 0 | 0 → 0 |

Historical pre-review candidate (different source; not an additional sample of
this candidate):

| Workload | ns/op | B/op | Allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkParseDecimal/minimum-four-digit` | 320 | 24 | 1 |
| `BenchmarkParseDecimal/short-two-digit` | 145.7 | 4 | 1 |
| `BenchmarkParseDecimal/overfull-fraction` | 281.3 | 56 | 2 |
| `BenchmarkFormatDecimal` | 161.1 | 24 | 1 |
| `BenchmarkAmountJSON/encode-maximum` | 3207 | 232 | 10 |
| `BenchmarkAmountJSON/decode-maximum` | 7035 | 1163 | 22 |
| `BenchmarkCheckedAdd` | 43.09 | 0 | 0 |
| `BenchmarkParseCode` | 55.02 | 0 | 0 |

Formatting changes from 72 to 24 B/op and three to one allocation.
Every case exceeded 30 timed seconds. Nominal budget is **240s per phase**;
process wall is **291.005s baseline /
291.629s candidate**. Wall/per-case-flag ratios are
**9.70x / 9.72x**;
wall/phase-budget ratios are **1.21x /
1.22x**.

These are single shared-host observations, not distributions or an overall
speedup claim. Slower samples, including unchanged paths, remain visible.
No timing regression is dismissed as proven noise. The candidate host sample
is retained; baseline-start and continuous governor parity were not observed,
and no affinity or power isolation was imposed.

Relevant aggregate profile rows:

```text
baseline/cpu: 30.20s  9.81%  9.81%     30.22s  9.82%  github.com/deliri/primitive/v2026/currency.currencyDefinitions
baseline/cpu: 9.86s  3.20% 32.77%      9.86s  3.20%  github.com/deliri/primitive/v2026/currency.accumulateDecimal
baseline/cpu: 3.39s  1.10% 47.40%     64.13s 20.84%  github.com/deliri/primitive/v2026/currency.decimalDigits
baseline/cpu: 0.65s  0.21% 75.87%     34.05s 11.06%  github.com/deliri/primitive/v2026/currency.Amount.Decimal
baseline/cpu: 0.47s  0.15% 77.72%      8.28s  2.69%  github.com/deliri/primitive/v2026/currency.fixedExponentDecimal
baseline/mem: 4.01GB 10.94% 36.75%    13.48GB 36.75%  github.com/deliri/primitive/v2026/currency.decimalDigits
baseline/mem: 3.72GB 10.15% 57.07%    11.12GB 30.32%  github.com/deliri/primitive/v2026/currency.Amount.Decimal
baseline/mem: 3.67GB 10.01% 67.07%     3.67GB 10.01%  github.com/deliri/primitive/v2026/currency.fixedExponentDecimal
review-candidate/cpu: 28.69s  9.45% 21.67%     28.69s  9.45%  github.com/deliri/primitive/v2026/currency.currencyDefinitions
review-candidate/cpu: 10.87s  3.58% 34.68%     10.87s  3.58%  internal/strconv.ParseUint
review-candidate/cpu: 3.30s  1.09% 55.62%     13.64s  4.49%  strconv.ParseUint
review-candidate/cpu: 2.60s  0.86% 62.28%     65.59s 21.59%  github.com/deliri/primitive/v2026/currency.decimalDigits
review-candidate/cpu: 1.50s  0.49% 72.75%     34.76s 11.44%  github.com/deliri/primitive/v2026/currency.Amount.Decimal
review-candidate/cpu: 0.22s 0.072% 83.54%      9.76s  3.21%  internal/strconv.AppendInt
review-candidate/cpu: 0.21s 0.069% 83.60%      9.97s  3.28%  strconv.AppendInt (inline)
review-candidate/mem: 4.34GB 20.35% 49.62%     4.34GB 20.35%  github.com/deliri/primitive/v2026/currency.Amount.Decimal
review-candidate/mem: 2.95GB 13.81% 63.44%     9.20GB 43.09%  github.com/deliri/primitive/v2026/currency.decimalDigits
```

The original profile shows string concatenation and the three formatting
allocations, plus the handwritten accumulation work. The candidate delegates
numeric conversion to Go and keeps only bounded currency projection.
Aggregate profile percentages do not establish the cause of each timing delta.
alloc_space is cumulative sampled allocation volume, not retained heap or peak RSS.

## Semantic fuzz execution

All five targets ran serially after the benchmark phase with -run=^$,
-fuzz=^<target>$, -fuzztime=30s, -parallel=4, -fuzzminimizetime=1s,
-timeout=2m. Persistent caches were not cleared; baseline gathering and complete
progress output remain in the raw record. No target failed, retried or skipped.

| Target | Executions | Process wall | Result |
| --- | ---: | ---: | --- |
| `FuzzDecimalParserAgainstStandardGrammarAndBigRationalOracle` | 1,633,992 | 33.002s | passed |
| `FuzzParseCodeAgainstClosedCurrencyDomain` | 1,843,838 | 30.466s | passed |
| `FuzzCodeJSONAgainstIndependentStringTokenOracle` | 1,625,092 | 30.579s | passed |
| `FuzzAmountJSONAgainstStandardTokenStreamOracle` | 1,439,092 | 30.566s | passed |
| `FuzzAmountNominalArithmetic` | 1,481,944 | 30.452s | passed |

Total: **8,023,958 executions**.
Nominal budget 150s, process wall **155.066s**;
wall/per-target flag **5.17x**, wall/phase budget **1.03x**.
No failing corpus was produced.

## Review artifacts

- [Machine evidence](currency_upgrade_20260909_evidence.json) retains every run,
  exact argv, source binding, environment, counts, exit, failures and artifacts.
- Raw evidence: `/home/d/engineering-evidence/primitive/currency-upgrade-20260909`.
- All three CPU/memory profile pairs and corresponding binaries remain outside Git.
- 4,285 distinct source snapshots and all retained run artifacts were
  rehashed by the report builder. The builder is retained and reproducible;
  it is an inventory tool, not an independent acceptance authority.

Currency is ready for user review before a release checkpoint. ID is next in
the saved queue after Currency approval.
