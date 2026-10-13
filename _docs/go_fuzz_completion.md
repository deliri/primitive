# Native Go fuzz completion observation

`runprotocol.ObserveGoFuzzCompletion` observes the exact top-level terminal
frame emitted by Go's `testing.F.report`. The caller supplies a validated
`GoFuzzTarget` and borrows the completed stdout source after native wait.
Package PASS, aggregate executions, subtarget completion and process exit zero
do not imply completion of the selected target. Absence, pass, skip and failure
remain distinct closed values. Duplicate selected terminal frames refuse.

Go's native identifier grammar and cmd/go's Fuzz-name rule admit the target.
Primitive delegates physical framing to Lineio and matching to Go regexp.
The observer consumes the complete source, discards partial observations on
native read or cancellation failure, and retains only the selected record's
native byte range. It borrows no source beyond the call. Working memory is
fixed relative to transcript extent, plus the configured target/pattern; disk
capture growth belongs to the caller's native scratch lifetime. Plain testing
output is an observation of a producer, not cryptographic attestation.

Furnace receipts are retained under
`/data/evidence/raw/primitive/2026-10-13-fuzz-completion`. Native production tests
execute actual Go fuzz targets, including no seeds, skip, missing selection,
sibling selection and a failed seed. Source-pressure tests retain native causes
even when a source also yields valid-looking terminal bytes. A deliberate
skip-to-pass projection mutation fails the actual native skip case. Full
runprotocol restoration, independent physical-record and identifier semantic
fuzzing, and three-second transcript-window benchmarks are retained there.
The earlier missing-ingress inventory failure remains a separate attempt.

This is a narrow native observation boundary. It does not own Witness policy,
process supervision, corpus preservation, or the rest of Witness's live fuzz
classifier. It does not close Native JSON buffering or certify another
platform's runtime behavior. Independent acceptance belongs to the user.
