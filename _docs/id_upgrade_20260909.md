Approved checkpoint: the user instructed “go on” after the ID review surface; releasing as v2026.1.34. Original execution report follows unchanged.

# ID upgrade — ready for review — 2026-09-09

Base: v2026.1.33, a4fe42bbe45c9b05c9f176c8f400e91acfb1075b.
Furnace: Go 1.27.1 linux/amd64; GOWORK=off; GOMAXPROCS=8.
Candidate Go-source digest: b097aa281d24ae88492eaf2364c3a6cf071a635776376f4991e99b56afdc6ce7.
The complete 2,214-line local testing protocol was read during this sweep and
verified unchanged before ID test edits. This is author-produced execution
evidence awaiting review, not independent acceptance. ID is uncommitted.

## Production change

The earlier ID notes explicitly covered only a bulk reservation audit with no
production change. This is its first complete package sweep in the saved queue.

Two reproduced defect classes crossed three boundaries:

- Both JSON decoders decoded arbitrarily large input before applying their
  fixed identity grammar. UUIDv7JSONMaximumBytes now caps complete input at
  218 bytes and ULIDJSONMaximumBytes at 158, before calling Core. Every canonical
  text byte can be a six-byte Unicode escape; whitespace consumes the same
  budget. Core still owns strict JSON syntax. Fresh/populated receivers stay
  unchanged on refusal, retaining Core JSON and ID error identities.
- Request.Validate accepted negative wall observations even though both
  constructors refused them. Validation now uses the existing observedMilliseconds
  owner for the shared epoch rule. The corrected Temporal wall remains authoritative.
  A valid shared Request can still produce an unset ULID at epoch with an all-zero
  entropy head: NewULID owns that value-specific refusal, while UUID version/variant
  marks make the same request constructible as UUIDv7.

The compiled red run failed six named rows: two negative observations and four
oversized JSON variants. Original production files matched the clean snapshot;
only new bound constants were declared before red. An initial test-build attempt
had a signed/unsigned Temporal-constant mismatch. It is retained and excluded
from behavioral-red accounting.

Profiles identified UUID parsing's redundant render-and-compare allocation.
ID now checks exact text length and uppercase hex, delegates decoding to Go's
uuid.Parse, then validates version/variant. The unused compact array and three
local scanning helpers were removed. Go owns UUID encoding/decoding; ID adds
closed admission. ULID's bounded two-word Crockford codec is unchanged.

Production changes are limited to request.go, uuid.go, ulid.go and new json.go
constants. No new production structs, dependencies, registries, goroutines,
sequencers, pools or clock/entropy acquisition were added. This remains a pure,
fixed-size value boundary with caller-provided observation and secret material.

## Boundary inventory and hostile proof

| Boundary / layer | Proof |
| --- | --- |
| Request.Validate / NewUUIDv7 / NewULID | Epoch, submillisecond and int64 extremes; invalid observations/entropy extents; destroyed handles; independently computed timestamp/entropy bytes; corrected wall |
| Entropy ownership | All 128 material bits varied: 80 ULID and 74 UUID bits retained; unused six-byte tail excluded; repeated construction exact; caller material preserved |
| Destruction race | Two stamp extremes, 32 races each; joined goroutines; exact success or typed zero refusal under any schedule; all later mints refuse destroyed entropy |
| Text parsers | Every byte value at every position; independent Go UUID parse/render and big.Int ULID oracles check both acceptance and rejection |
| ULID byte constructor/projection | All 128 single set bits and complements; independent big.Int/base32 arithmetic; input and returned arrays cannot mutate retained value; zero refuses |
| String / AppendText / validity | Goldens retained; exact/one-short/one-spare capacity; prefixes/guard bytes preserved; reused backing storage; invalid values cannot project plausible output |
| JSON | Direct below/at/above byte ceilings, fully escaped tokens, UTF-8/surrogate refusal, trailing/concatenated documents, neutral null, unchanged receivers, exact canonical output |
| Compiler / architecture | Three production structs classified; exact public/import surface; no aliases/maps; seven ingress functions mapped to six compiled semantic fuzz owners |
| Effect acquisition | Compiled UUID whitelist: UUID, Parse, Nil. Temporal whitelist: Observation and NanosecondsPerMillisecond. New generator/Observe selectors fail tests |
| Allocations | Core/testserial RuntimeAllocation declarations; zero-allocation ratchets for both parsers and both exact-capacity appenders |

Canonical grammar, golden text/wire and ordering tables remain. Twelve older
single-case/weaker checks were replaced by stronger tables. Two parallel tests
that ran testing.Benchmark became serial AllocsPerRun tables. Construction now
uses an actual triad table; JSON and request admission have named local triads.
This pure value package has no durable writer, OS effect, ledger/verifier or
producer/classifier evidence handoff, so those layers and a synthetic 50-row
handoff are not fabricated.

Architecture scans use compiler-embedded source and in-memory synthetic fixtures,
removing direct test filesystem operations. The ingress inventory no longer
omits constructors or discovers only already-known parser names. Semantic fuzz
checks independently classify admission; blanket typed refusal cannot pass.
Separate JSON targets avoid selector bytes that discard most generated inputs.
Test/benchmark fixture secret material now has explicit destruction.

## Validation and attempts

Original race: 176 passing events, zero
failures/skips, 96.8% coverage.
Final race: **609 passing events**, **44 top-level
functions**, zero failures/skips, **97.7% statement coverage**.

Scoped go fix -diff, Vet, Staticcheck, Errcheck, Witness and production gocyclo <=10
pass. go fix suggested no changes. macOS arm64 and Windows amd64 test binaries
compile; they were not executed. Full-module gates and consumer updates were not run.

Nine mutations compiled and failed selected tests: removed JSON bound, skipped
epoch validation, blanket UUID refusal, broken ULID half carry, lost entropy
tail byte, untracked ingress, restored allocating UUID parser, introduced
uuid.NewV7, and introduced temporal.Observe. All were restored. Generator mutations
added unreachable private functions; the scanner caught them without effects.

One Witness pass found 23 terse failure diagnostics. Actual/expected values were
added before final checks. The constant signedness build failure, compiled red,
intermediate checks, all nine mutations and measurements remain in the manifest.

The final audit corrected the old claim that imports alone prove purity:
uuid and Temporal expose acquisition APIs too. Compiled selector inventories and
mutations now protect the exact pure APIs ID uses. That audit and fixture cleanup
changed tests after the first candidate measurements. Production stayed identical.
Final checks, benchmarks, profiles and fuzz all match the final Go-source digest.
Earlier candidate results remain a separate historical source, never a best-run
selection or substitute for final-source evidence.

## Benchmarks and profiles

Every revision used the same ten checked workloads, one pass, 30s per case:

    go test -run=^$ -bench=. -benchmem -benchtime=30s -count=1 -timeout=12m
      -cpuprofile=<phase>/cpu.pprof -memprofile=<phase>/mem.pprof
      -o <phase>/id.test ./id

The benchmark file is byte-identical across all phases. Fixture values and timed
work are unchanged. The helper delta is SecretMaterial.Destroy cleanup outside
timed work; removing that exact block reconstructs the original testEntropy and
testRequest source byte-for-byte.

| Workload | Original ns/op | Earlier candidate ns/op | Final ns/op | B/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| BenchmarkParseULID | 155.6 | 157.2 | 157.4 | 0 → 0 | 0 → 0 |
| BenchmarkParseUUIDv7 | 413.8 | 86.02 | 84.13 | 48 → 0 | 1 → 0 |
| BenchmarkULIDAppendTextReusedBuffer | 60.17 | 59.71 | 67.9 | 0 → 0 | 0 → 0 |
| BenchmarkNewUUIDv7 | 378 | 432.6 | 376.3 | 16 → 16 | 1 → 1 |
| BenchmarkUUIDv7JSON/encode | 986.5 | 950.1 | 989.5 | 128 → 128 | 4 → 4 |
| BenchmarkUUIDv7JSON/decode | 970.3 | 501.2 | 499.7 | 64 → 16 | 2 → 1 |
| BenchmarkNewULID | 388.3 | 461 | 424.3 | 16 → 16 | 1 → 1 |
| BenchmarkULIDJSON/encode | 963.7 | 941.8 | 989.3 | 96 → 96 | 4 → 4 |
| BenchmarkULIDJSON/decode | 552.4 | 573.6 | 580.8 | 16 → 16 | 1 → 1 |
| BenchmarkUUIDv7AppendTextReusedBuffer | 43.17 | 34 | 34.11 | 0 → 0 | 0 → 0 |

Every row exceeded 30 timed seconds. Nominal budget: 300s per revision.
Process wall: **367.189s original / 365.743s earlier
candidate / 360.379s final**.
Wall/30s-flag ratios: 12.24x / 12.19x /
12.01x. Wall/300s-budget ratios: 1.22x /
1.22x / 1.20x.

UUID parse: 48 → 0 B/op, 1 → 0 allocations. UUID JSON decode: 64 → 16 B/op,
2 → 1 allocations. Appenders and ULID parse stay allocation free. Construction
retains its bounded Core entropy copy.

One sample per workload/revision is not a distribution or an overall speedup.
Slower construction and other samples remain visible. Furnace was shared,
without affinity/power isolation. Candidate governor observations are retained;
baseline-start and continuous power parity were not captured. Timing differences
are not conclusively attributed to code or host noise. These are observations
and allocation ratchets, not statistical performance acceptance.

Selected aggregate profile rows:

    baseline/cpu: 16.95s  4.50% 37.20%     20.18s  5.36%  github.com/deliri/primitive/v2026/id.compactCanonicalUUIDText
    baseline/cpu: 2.96s  0.79% 62.72%     37.19s  9.87%  github.com/deliri/primitive/v2026/id.NewUUIDv7
    baseline/cpu: 2.93s  0.78% 64.28%     21.99s  5.84%  github.com/deliri/primitive/v2026/core.SecretMaterial.CopyBytes
    baseline/cpu: 2.93s  0.78% 65.06%     32.49s  8.62%  github.com/deliri/primitive/v2026/id.NewULID
    baseline/cpu: 2.21s  0.59% 71.15%     54.37s 14.43%  github.com/deliri/primitive/v2026/id.ParseUUIDv7
    baseline/cpu: 1s  0.27% 80.96%     11.92s  3.16%  uuid.UUID.MarshalText (inline)
    baseline/cpu: 0.65s  0.17% 83.40%     22.29s  5.92%  uuid.UUID.String
    baseline/cpu: 0.05s 0.013% 85.82%      5.16s  1.37%  github.com/deliri/primitive/v2026/id.UUIDv7.String
    baseline/mem: 7.23GB 40.66% 40.66%     7.23GB 40.66%  uuid.UUID.String
    baseline/mem: 2.78GB 15.63% 56.30%     2.78GB 15.63%  github.com/deliri/primitive/v2026/core.SecretMaterial.CopyBytes
    baseline/mem: 0     0% 99.88%     1.28GB  7.20%  github.com/deliri/primitive/v2026/id.NewULID
    baseline/mem: 0     0% 99.88%     1.50GB  8.43%  github.com/deliri/primitive/v2026/id.NewUUIDv7
    baseline/mem: 0     0% 99.88%     5.61GB 31.53%  github.com/deliri/primitive/v2026/id.ParseUUIDv7
    baseline/mem: 0     0% 99.88%     1.62GB  9.13%  github.com/deliri/primitive/v2026/id.UUIDv7.String
    closure/cpu: 5.82s  1.59% 49.25%     45.51s 12.44%  github.com/deliri/primitive/v2026/id.ParseUUIDv7
    closure/cpu: 4.27s  1.17% 58.99%     23.38s  6.39%  github.com/deliri/primitive/v2026/core.SecretMaterial.CopyBytes
    closure/cpu: 3.01s  0.82% 65.56%     38.44s 10.51%  github.com/deliri/primitive/v2026/id.NewULID
    closure/cpu: 2.42s  0.66% 72.26%     37.06s 10.13%  github.com/deliri/primitive/v2026/id.NewUUIDv7
    closure/cpu: 0.29s 0.079% 86.59%      3.27s  0.89%  uuid.UUID.MarshalText (inline)
    closure/cpu: 0.15s 0.041% 86.95%      5.69s  1.56%  github.com/deliri/primitive/v2026/id.UUIDv7.String
    closure/cpu: 0.09s 0.025% 87.12%      5.38s  1.47%  uuid.UUID.String
    closure/mem: 2836.04MB 22.56% 22.56%  2836.04MB 22.56%  github.com/deliri/primitive/v2026/core.SecretMaterial.CopyBytes
    closure/mem: 1681.08MB 13.38% 73.33%  1681.08MB 13.38%  uuid.UUID.String
    closure/mem: 0     0% 99.85%  1377.52MB 10.96%  github.com/deliri/primitive/v2026/id.NewULID
    closure/mem: 0     0% 99.85%  1458.52MB 11.60%  github.com/deliri/primitive/v2026/id.NewUUIDv7
    closure/mem: 0     0% 99.85%  1681.08MB 13.38%  github.com/deliri/primitive/v2026/id.UUIDv7.String

All three CPU/memory profile pairs and matching binaries remain outside Git.
alloc_space is cumulative sampled allocation, not retained heap or peak RSS.
Aggregate profile percentages do not explain every per-case timing difference.

## Final semantic fuzz

Six targets ran serially after final benchmarks/profiles with -run=^$,
-fuzz=^<target>$, -fuzztime=30s, -parallel=4, -fuzzminimizetime=1s, -timeout=2m.
Persistent caches were retained and gathering/progress output recorded.
No target failed, skipped or retried after failure.

| Target | Executions | Process wall | Result |
| --- | ---: | ---: | --- |
| FuzzParseUUIDv7 | 2,025,822 | 31.366s | passed |
| FuzzParseULID | 1,956,608 | 30.467s | passed |
| FuzzUUIDv7JSON | 1,699,879 | 30.431s | passed |
| FuzzULIDJSON | 1,712,235 | 30.481s | passed |
| FuzzULIDFromBytes | 1,612,474 | 30.357s | passed |
| FuzzIdentityRequest | 1,753,460 | 30.428s | passed |

Total: **10,760,478 final-source executions**.
Nominal budget 180s; process wall **183.530s**.
Wall/per-target flag 6.12x; wall/phase budget 1.02x.
No failing corpus was produced. Earlier-source fuzz is retained separately.

## Review artifacts

- [Machine evidence](id_upgrade_20260909_evidence.json): all commands, source
  bindings, tool identity references, environment, results and artifact hashes.
- Raw root: /home/d/engineering-evidence/primitive/id-upgrade-20260909.
- 4,287 distinct source snapshots and all run artifacts rehashed.
- Large profiles, binaries, logs and snapshots stay outside Git.
- The retained builder checks inventory integrity; it is not independent acceptance.

ID is ready for review before bump/commit/push. Distribution follows after approval.
