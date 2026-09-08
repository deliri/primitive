# Controlwire package upgrade — September 8, 2026

Status: reviewed and approved by the user for publication in v2026.1.22.
The supplied Grok review reported no correctness, protocol-invariant, or security
defects in the production changes. The evidence manifest retains its original
pre-review capture status.

## Production changes

Controlwire still owns blind mechanical agreements: nominal nonces and secrets,
closed route/revision pairs, exact replay commitments, and typed Exchange calls.
No product policy, network runtime, replay store, or workflow machinery was added.

- All eight scalar JSON ingress methods enforce Core's document ceiling before
  decoding, including direct calls to `UnmarshalJSON`. Cursor and replay-identity
  documents retain their existing bounded decoders. Valid JSON whitespace below
  the ceiling remains accepted; refusal preserves a populated receiver.
- Nonce/verifier text parsing decodes into a fixed array through
  `core.DecodeCanonicalHex`, which delegates decoding to Go's `encoding/hex`.
  Baseline allocation profiles identified the old string/slice conversions and
  allocated hex output. No second codec was introduced.
- Replay commitment checks the request-owned byte budget and requires one valid
  JSON object before hashing. Go `jsontext.Value` owns syntax, UTF-8, duplicate
  names and document framing. The typed request continues to own its fields and
  canonical emission. Provider errors keep their Core identities.
- The HTTP sender passes the request-owned byte ceiling into Exchange. A real
  local HTTP fixture proves refusal before authority execution. A nil interface
  request is refused before invoking its methods.
- Invalid route capabilities and failed final receive validation return zero
  output. A deliberately changing producer proves that the final validation
  cannot leak an apparently usable received body/replay assessment.
- A corrupt private support count is refused before indexing its fixed storage.
  Membership remains a bounded fixed array; there is no new set implementation.

The default shared JSON ceiling is deliberately reused. This is bounded
admission, not a new per-scalar wire protocol. HTTP still executes through
Exchange. Production has no filesystem or clock effects; revised fixture file
reads use Filestore and the observation backstop uses Temporal.

## Test and fuzz pressure

The complete local `_docs/testing_protocol.md` (2,214 lines) was read before test
changes. Its SHA-256 is recorded in the evidence manifest. Existing quota padding
was removed from support, origin, replay representation, and socket tables.

- Every one of the 4,095 nonempty published support subsets is tested against all
  12 candidates, including reversed construction, unchanged caller storage, and
  input clearing after ownership transfer. These 49,140 membership decisions
  exhaust the bounded domain rather than repeating selected list sizes.
- Route and support-outcome tests exhaust all 256 backing bytes against explicitly
  published enum members. Nil receivers and document ceilings are exercised for
  all ten JSON receivers, including exact maximum-minus-one/maximum/maximum-plus-one.
- Real RegistrationRequest and authenticated response fixtures exercise the
  Controlwire → Exchange ingress/egress boundary. Changed body limits are proved
  with HTTP call counts; failed final validation is proved with exact zero output.
- JSON and text fuzz callbacks contain their own verdicts. Independent Go hex,
  JSON and big.Int references catch incorrect acceptance, incorrect refusal and
  changed facts. JSON integer comparisons explicitly preserve uint64 precision.
- Valid fuzz seeds come from typed production values. Whitespace acceptance is
  compared through canonical fixed points and exact facts. The former cursor and
  token fuzz helpers incorrectly equated admitted JSON bytes with canonical bytes.
- Compiler-visible inventories cover ten JSON decoders and seven text parsers;
  the source ratchet checks JSON receiver declarations. Struct classification now
  follows local named projections, including `policyCursorWire`; synthetic
  matcher cases cover direct, named, aliased, generic and cyclic declarations.
- Secret fixtures and successful decoded tokens release owned handles. The token
  formatting test no longer guesses that short hex substrings cannot appear in a
  runtime pointer address.

The eight mutation experiments each produced a behavioral test failure: disabled
nonce acceptance, removed JSON ceiling, acceptance of absent support pairs, lost
replay separator, accepted replay conflict, altered policy-ID bits, and cursor/
replay parsers that always refuse. Their overlays and failing output are retained.
They are mutation evidence, not eight additional production bugs.

## Verification and limits

All eleven fuzz targets passed their serial 30-second budgets, totaling
2,699,088 executions. Cache and corpus snapshots are retained.

The final race run passed 5,255 leaf tests (5,318 pass events including parent
subtests), with no skipped tests and 88.2% statement coverage. Original package
coverage was 85.9%, with 617 passing leaf tests. Coverage is not a claim of complete
branch proof: several retained branches defend impossible failures of already
validated constants/digests, and OS entropy failures are not injected through
process-global replacements. Public negative results and boundary ownership are
checked directly; no production seams were added to chase a percentage.

Scoped `go vet`, Staticcheck, Errcheck, Nilaway, Witness-lint, `go fix -diff`,
Gocyclo (maximum 10) and Goconst (minimum 4 characters / 3 occurrences, no tests)
completed successfully. `deadcode -test ./controlwire` has no Controlwire
findings; it also reports unrelated dependency APIs outside this package's test
binary reachability. Full-module gates were not rerun.

After fuzzing, the sender-budget table was tightened to require both
`ErrExchangeRequest` and `ErrJSONContract`; the full race suite passed again.
No production, fuzz callback, or benchmark implementation changed in that final
assertion-only adjustment.

macOS/arm64 tests ran natively under Go 1.27.1. Linux/amd64 and Windows/amd64 are
compile checks, not native execution or platform-equivalence claims.

## Benchmark method

Ten fixed workloads, one sample per workload per side, requested 30 seconds,
serial execution, `GOWORK=off`, Darwin/arm64 Apple M1 Max, AC power, Go 1.27.1.
Every sample retains raw output, actual iteration count, effective timed duration,
command duration, CPU profile, memory profile and the exact test binary.

The after measurements restore **all baseline test sources** with a Go overlay,
including the unchanged benchmark implementation and original fixture setup.
Current fixture-cleanup and Filestore-read changes therefore cannot masquerade
as production performance improvements. The final post-loop nil guard is also
excluded from both comparison sides. HTTP benchmarks explicitly include fresh
httptest request/recorder construction in each measured operation; they are
in-process socket workloads, not TCP latency measurements. The signed response
workload includes the real Controlplane response's validation and marshaling.

Single-machine samples do not establish a timing distribution. In particular,
unchanged token lifecycle code can vary in elapsed time. Fixed allocation changes
and profiles support the local nonce-decoder conclusions; no timing threshold or
broad speedup percentage is claimed.

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| nonce-text | 110.3 | 69.60 | 96 | 0 | 2 | 0 |
| nonce-json | 293.3 | 204.9 | 112 | 16 | 3 | 1 |
| nonce-json-oversized | 1215535 | 60.00 | 1057541 | 112 | 10 | 4 |
| token-lifecycle | 545.4 | 397.0 | 128 | 128 | 2 | 2 |
| cursor-round-trip | 2032 | 1490 | 939 | 939 | 24 | 24 |
| support-construct | 331.8 | 280.2 | 0 | 0 | 0 | 0 |
| support-assess | 141.9 | 124.2 | 0 | 0 | 0 | 0 |
| replay-commit | 8878 | 9011 | 7460 | 7462 | 115 | 115 |
| socket-receive | 36895 | 38022 | 33401 | 33305 | 426 | 424 |
| socket-write | 1894985 | 1782074 | 987089 | 987368 | 13876 | 13870 |

The before nonce allocation profile attributes essentially all hot-loop bytes to
Core's old allocating digest text path; after, the nonce workload is allocation
free and CPU samples are in canonical admission and Go hex decoding. Oversized
JSON's before allocation profile is dominated by Go's string decoding; after,
all sampled hot-loop allocations are the typed `errors.Join` result. The fixed
input is roughly one MiB, so the new rejection cost is independent of its bytes.

Replay and receive now perform an additional Go JSON syntax check over the
request-owned bounded document. Their single timing samples are slightly higher;
those costs and unchanged/slightly changed allocation counts are retained above.
The large response-writing footprint is dominated by the supplied Controlplane
response's commitments, strict decoding, and canonical emission. This review
preserves those checks and leaves that dependency's optimization to its package
review.


## Evidence retention

[upgrade_evidence.json](upgrade_evidence.json) binds every recorded command, failed attempt, source
snapshot, overlay, fixture, benchmark, profile/binary pair, fuzz cache/corpus and
artifact hash. The final audit verified 124 commands, 1,706 source snapshots,
526 artifacts and 4,577 content digests with zero mismatches. Repeated fuzz-cache
states use indexed snapshots, keeping the complete report about 3 MB.
Raw evidence remains local and ignored under
`controlwire/testdata/test-upgrade-20260908/`; large binaries/profiles are not staged.

Retained failures are explained rather than discarded:

- The initial emission/extent tests reproduced 19 failing public boundary rows;
  the corrected client fixture reproduced three sender-budget failures, and the
  final-binding test reproduced partial-output leakage. The corrupt private
  count separately reproduced an indexing panic, and an absent interface request
  separately reproduced a nil-method panic. These rows group into several
  defect classes; they are not advertised as 24 independent regressions.
- The first client fixture overrode emission without a paired decoder. Exchange
  correctly refused its positive control. That fixture was replaced with a real
  request's paired codec; the corrected old-source overlay and current source
  respectively fail and pass the intended budget assertions.
- A new whitespace seed fails the old cursor oracle. This is a test defect,
  distinguished from the production red cases.
- Witness-lint required the existing external-wire annotation for Go jsontext
  syntax validation. Staticcheck and Nilaway found inventory/benchmark harness
  issues; corrected runs are retained. Removing padded cases briefly left an
  unused import; the compiler failure and corrected run are retained.
- Running the entire restored old test suite encountered its old disk-reading
  source inventory against the new on-disk inventory declaration. The benchmark
  harness was subsequently compile-checked with `-run=^$`; actual final-source
  tests passed separately. Benchmarks do not execute source-inventory tests.
- The original test-only capture predates adding Controlplane fixture-file
  binding to the recorder. Every benchmark and retained test command binds those
  fixture files. This does not affect the benchmark comparison.

Local evidence is not independent Anvil acceptance. The user approved this package
and its release checkpoint. Keygen is next in the recorded usage-priority order.
