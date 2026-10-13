# Native Go fuzz metric records

`runprotocol.GoFuzzMetricRecords` lends one completed physical record at a time.
Its request borrows a native seekable/positional source. The record retains four
fixed signed counter fields, their closed presence dispositions, provider
cumulative/failing-input observations and original physical coordinates.

Primitive owns framing, bounded native reads, integer normalization, Go regexp
matching and stdlib strconv admission. Leading whitespace, leading zeroes,
formatting commas, unknown text and physical records have no extent quota.
Integer representability remains a real signed 64-bit schema constraint. There
are several native passes over a record; this is bounded working memory rather
than a single-pass throughput claim. Scratch/source storage follows input size.

Absent and malformed fields are distinct. A malformed field carries the stable
Core output refusal and preserves a native strconv cause when one was produced.
Read/context failures yield no record alongside their error. A yielded prefix
is provisional until the caller finishes the complete source and native cleanup.
No reader or root escapes in a record.

Witness owns counter policy: clamping signed observations, cumulative versus
delta growth, explicit-crasher precedence, checked arithmetic, selected-work
admission and evidence classification. These records observe supplied process
text; they do not attest that a printed count equals actual executed work.

Furnace proof is retained under
`/data/evidence/raw/primitive/2026-10-13-fuzz-metric`. Actual Go completed,
failed-seed and skipped targets supply native producer proof. Signed extrema,
numeric refusals, closed enum domains, original causes and fields beyond the
former Witness limit are separate scopes. Erasing a native scalar reproduces a
real coordinator refusal and grammar failures. Independent numeric and physical
record/field oracles use three-second development fuzz budgets.

The full runprotocol candidate race scope retains 1,031 passing terminal events,
zero failures/build failures/skips. Later test additions and source changes are
bound by their own receipts rather than silently upgrading that earlier scope.
Native working-memory benchmarks and immutable committed proof are retained
separately. This capability does not certify all Primitive execution, Witness's
remaining signal/stack/path owners, final migration gates or user acceptance.
