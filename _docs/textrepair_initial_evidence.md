# Initial textrepair capability evidence

Production parent: 48b73c1ed4cb9f9513ea2f9134d23b8a66887647.
The initial core placement was withdrawn: exported core contracts require two
named Primitive consumers. The dedicated textrepair package owns the operation
and request; core receives only its catalog identity. No fabricated consumers
or core-export waiver were introduced. The catalog enum appends an identity,
preserving existing numeric values; the fixed admission array grows by one for
that actual identity. Public fields and Validate have documentation.

This is a new capability ratchet derived from Blink Prism's bounded repair
contract. The deliberate mutation below establishes that preservation of a
genuine U+FFFD is load-bearing. No Blink migration or dependency update is
claimed by this Primitive slice.

External receipts: /Users/d/code/.work-evidence/anvil-execution-<id>/receipt.json.
All runs used GOWORK=off, Go1.27.1 darwin/arm64 and -count=1. Receipts retain
revision/dirty source digests, complete argv, outputs with bytes/digests and
process status. Planned not-run/unavailable denominators are not established
by this local capture harness; these are not independent acceptance receipts.

- 1Lv9v3: `go test -json ./textrepair ./core ./capabilities -count=1
  -timeout=5m`; core failed to compile because the catalog admission array
  count was not incremented. 11441 test events passed in the other packages,
  two packages passed, one failed, exit 1. Compile failure was retained.
- FA8JEJ: same command after correcting that array; 13420 passed events,
  two failed, two packages passed, one failed, exit 1. The two failures are
  TestRealWorldEffectOwnershipMatchesLandedProduction and
  TestPrimitiveJSONMarshalersHaveCompleteValidationWitnesses, both separately
  reproduced on clean unchanged parent in receipt dQTxqB. They remain open.
- v8dfE7: changed `r == utf8.RuneError && size == 1` to
  `r == utf8.RuneError`, discarding genuine replacement characters. Command
  `go test -json ./textrepair -run '^TestUTF8PrefixLayerTriad$' -count=1
  -timeout=3m`; five passed and two failed events (child and parent), exit 1.
  Mutation discarded before all following runs.
- FmuJYn: serial `go test -json ./textrepair -run '^$' -bench
  '^BenchmarkPrefixMalformedPrefix$' -benchtime=30s -count=1 -timeout=3m`;
  exit 0. Fixed workload: 128 repetitions of malformed-byte/a/é/界/😀,
  output ceiling 1024 bytes. Result is observed, setup proves nonempty output,
  and allocations are reported. 9,019,249 iterations, 3937 ns/op, 3320 B/op,
  nine allocs/op. No code-only improvement, profile explanation or independent
  benchmark acceptance is claimed; this is one local sample without a baseline.
- GAQgMh: serial `go test -json ./textrepair -run '^$' -fuzz
  '^FuzzUTF8PrefixSemanticOrder$' -fuzztime=30s -parallel=1 -count=1
  -timeout=3m`; one passed fuzz event and package, exit 0. No crasher produced.

The new package still requires broader architecture/resource review, final
lint, independent acceptance and consumer migration. Baseline failures must
not disappear from the release gate. No release tag was created here.
