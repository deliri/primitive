Primitive v2026.1.122 adds Objects and a reusable ObjectEncoder for closed,
non-null, Go-declared struct records. Go owns the decoder, encoder, syntax,
UTF-8, duplicate-name checks and field matching. Primitive validates source,
destination, root type, required domain meaning, read/write counts and
cancellation. Root custom codecs are rejected so they cannot bypass the closed
declared shape. Nested nominal field codecs retain their owning contracts.

Memory follows one declared record and Go's scalar/parser buffers. There is no
raw document copy, per-record acceptance ceiling or retained sequence. Types
with collection fields retain those declared collections; scalar discovery
records are the intended consumer. This does not claim arbitrary-type O(1).
Failed writes can leave provisional bytes; callers must discard the output.

Development base: 31eca7e (full revision available in Git), dirty working tree.
Toolchain: go1.27.1 darwin/arm64. No independent acceptance is issued.

Commands and attempts:
- go test ./jsonio -run '^$': compiled, no behavioral tests.
- go test -race -count=1 ./jsonio -run '^TestJSONObject' -timeout=3m:
  initial shape tests passed. Adding hostile short writers exposed actual
  false success from Go's JSON encoder when a writer returned a short count
  with nil error. Primitive now enforces the count before completion.
  The focused suite passed after the fix, 1.148s.
- go test -race -count=1 ./jsonio -run '^TestJSONObjectStreamRetainedMemory' -timeout=3m:
  passed, 5.883s. Allocation isolation is declared through Primitive.
- go test -json -race -count=1 ./jsonio ./compass -timeout=3m:
  passed; complete development-race.jsonl retained, zero failures/skips.
  This is scoped package proof, not a repository-wide suite. Cache disabled
  by count; fuzz targets ran seeds only.
- go test -count=1 ./jsonio -run '^$' -fuzz '^FuzzJSONObjectEncoderExactMeaning$' -fuzztime=3s -timeout=3m:
  first development run passed, 197,871 executions / 4.162s. It overlapped
  the separate-process memory run and is retained as development-only evidence.
  The final serial run with the stronger independent canonical-byte oracle
  passed, 99,045 executions / 4.272s, 113 cached baseline seeds, 23 newly
  interesting inputs, ten workers. Complete semantic-fuzz.stdout retained.
- go test -count=1 ./jsonio -run '^TestJSONObjectEncoderPreservesWriteRefusals/(zero_acknowledged_bytes|partial_acknowledged_bytes)$' -timeout=2m:
  failed as required with an actual production mutation disabling short-write
  refusal. Both hostile cases failed. Mutation restored immediately; raw
  write-count-mutation.stdout retained.

The object stream, exact canonical bytes, null/unknown refusal, root codec
ownership, writer acknowledgement and sequence memory are new ratchets.
The final read helper was split after the race run without changing behavior;
exact-committed-source proof follows the source checkpoint. No lint gates ran.
