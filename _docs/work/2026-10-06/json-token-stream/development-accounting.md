Primitive v2026.1.121 adds jsonio.Tokens: a closed typed lexical projection of
Go jsontext.ReadToken. Go owns JSON grammar, duplicate-name refusal, UTF-8,
surrogates, decoder state and buffering. Primitive owns typed source admission,
declared nesting depth, cancellation, refusal identities and synchronous
backpressure. No document or array collection is constructed. Prefix tokens
are provisional until clean EOF. A source refusal emits one zero token and
ends the sequence; cancellation during a read is checked before EOF meaning.

Memory is independent of array item count. It includes Go's largest scalar
token buffer, nesting state and duplicate-name tracking for open objects.
This is not a claim of constant memory for arbitrary scalar lengths or
unbounded object field names. Witness's closed go-list field policy must
reject foreign fields and consume file-array items without retaining arrays.

Development base: c91dbbf2f51ca14240cfe333ea15b22bab25f6cf, dirty working tree.
Toolchain: go1.27.1 darwin/arm64. No independent acceptance is claimed.

Commands and attempts:
- go test ./jsonio -run '^$': compiled; no behavioral tests ran.
- go test -race -count=1 ./jsonio -run '^TestJSONTokens' -timeout=3m:
  initial failure was a fixture incorrectly treating adjacent JSON numbers
  '01' as a single invalid document. Go admits sequential root values.
  The hostile leading-zero fixture now uses '[01]', where grammar forbids it.
- go test -json -race -count=1 ./jsonio -run '^(TestJSONTokens|FuzzJSONTokensExactDecodedStrings)' -timeout=3m:
  passed, 456 events, zero failures/skips; focused test scope and seed fuzz only.
- go test -race -count=1 ./jsonio -timeout=3m: passed, 1.627s.
- go test -race -count=1 ./jsonio -run '^TestJSONTokensArrayRetainedMemory' -timeout=3m:
  passed, 1.648s; declared process allocation isolation through Primitive.
- go test -count=1 ./jsonio -run '^TestJSONTokensHostileGrammarAndDepth/above_depth$' -timeout=2m:
  failed as required with an actual production mutation disabling nesting
  refusal. Mutation restored immediately; depth-mutation.stdout retained.
- go test -json -race -count=1 ./jsonio ./compass -timeout=3m:
  passed. Complete development-race.jsonl has 1,651 events, no failures/skips.
  Retained bytes at 4,096 / 65,536 / 262,144 array items were 3,760 / -1,704 /
  3,728 against one fixed 262,144-byte allowance. Counts and exact values were
  verified from a fixed-state repeating reader without constructing the input.
- go test -count=1 ./jsonio -run '^$' -fuzz '^FuzzJSONTokensExactDecodedStrings$' -fuzztime=3s -timeout=3m:
  two development attempts passed: 249,752 executions / 4.135s, then 139,263 /
  4.120s after final cancellation behavior. Fuzz cache remained enabled.
- go test -count=1 ./jsonio -run '^$' -fuzz '^FuzzJSONTokensGrammarAndProjectionAgainstGo$' -fuzztime=3s -timeout=3m:
  passed, 424,256 executions / 4.206s, three baseline seeds and 171 newly
  interesting inputs, ten workers. Complete grammar-fuzz.stdout retained.
- go build ./jsonio: passed.

The token projection, owned validation, cancellation at read completion, exact
string bytes, bounded nesting and array retention are new ratchets. Grammar
and arbitrary-input projection are checked against the actual Go decoder,
which is the adapter's execution subject. No repository-wide suite, lint gate,
all-shape O(1) memory claim, Witness migration completion or acceptance is issued.
