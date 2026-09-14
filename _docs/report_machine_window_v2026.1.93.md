# Machine reporting window and authenticated report request

`permit.Terms.Reporting` carries a resolved machine reporting window inside the
existing signed permission. Zero is an absent reporting grant; partially set
windows fail validation. Product code selects policy. Primitive validates and
signs the values, without scheduling processes or maintaining fleet state.

`permit.ReportRequest` binds a signed report to an installation certificate.
Verification authenticates the authority certificate before trusting the device
key and then verifies the report. The receiving product must still check current
account/project authorization, revocation, timing, evidence custody and atomic
accounting. These helpers do not claim Firestore registration or ingestion wiring.

Local proof covers canonical admission, unchanged receivers on rejection,
request byte ceilings, authentic foreign certificate nomination, changed signed
facts, and positive/negative/absent reporting permissions. Deliberately accepting
a failed certificate verification fails the nomination test. Deliberately omitting
the reporting window from signed bytes fails the machine permission test. Both
mutations were restored; identities, source snapshots and results are retained.

The first changed-interval fixture was rejected structurally because its end was
before freshness. It was corrected to a structurally valid end so authentication
is exercised. This is a fixture correction, not a production red-state claim.

Fuzzing found that the expanded permission oracle omitted a genuine signed
no-reporting state retained in the corpus. Both genuine states are now generated
through production signing. The input remains at
`permit/testdata/fuzz/FuzzPermitDecodeSignedSemanticClosure/16b55bf900647951`.
The failed run is retained; the corrected fuzz run passed. This was an oracle
failure, not an authentication defect in production.

Final local permit tests recorded 253 passing test events, no failures or skips.
Focused vet, staticcheck, doctrine lint and the shared validation-witness ratchet
passed. Fuzz runs used 2 seconds plus 1 second minimization; observed wall time
also includes startup and baseline replay. No benchmark or performance claim is
made. The adjacent evidence index retains every completed attempt and the
recorder setup refusal. These are dirty-source development facts, not independent
acceptance. The previously recorded core GCS ownership-inventory finding remains
outside this slice; no full-package-sweep or complete-gate claim is made.
