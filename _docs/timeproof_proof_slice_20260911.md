# Timeproof proof slice — v2026.1.67

This is a scoped proof-quality repair, not Timeproof package-sweep closure.
Baseline source is v2026.1.66, 40db068aa8de68ebf157c9ca88d00e3b8c045ca4.

## Closed proof gaps

The old zero predicate ignored signer, serial, request digest, request nonce and
request authority. Five one-field fixtures failed before the repair. Complete
field comparison now lives in tests; the two incomplete production predicates,
which existed only for tests, are removed. An exact carrier-shape test requires
future fields to update the refusal predicate. This is a defect in the evidence
oracle; the red run does not claim that Verify actually leaked those fields.

A structurally admitted provider refusal reached Verify as ErrTimeProofRefused,
but the AuthorityEvidence fuzz oracle incorrectly required ErrTimeProofInvalid.
A typed custody fixture driven through real Verify and MarshalJSON proves the
red and green state. Refusals now require the stable Refusal type, valid
non-granting status, and exact zero proof. They are never counted as timestamp
success. The provider DER refusal is constructed with standard-library ASN.1;
it is an unsigned provider fixture, not an authentic granting response.

Every JSON selector now reaches one of the seven actual external decoders.
Pointer constraints let the compiler prove json.Unmarshaler support; runtime
any assertions are removed. Accepted canonical JSON must equal the admitted
source bytes, so an idempotently substituted valid document cannot pass merely
by reaching a marshal/parse fixed point.

Raw signed-response fuzzing covers both FreeTSA and DigiCert. Seeds come from
real Verify and typed evidence custody. Accepted proof fields and selected
signed TSTInfo, attributes, signature and signer must match the genuinely signed
seed agreement. The production parser locates the selected signed slices; the
oracle's expected agreement is the authenticated fixture, not a copy of parser
branches. Unsigned CMS framing is not misrepresented as signed content.

A separate targeted fuzz callback changes signatures, nonce, independent digest,
authority, or signed message-imprint binding while keeping the known authentic
response reachable. All mutations operate on canonical typed fixtures. Deliberate
production mutations bypassing signature verification and nonce matching each
fail this target and are discarded afterward. Existing named hostile tables
remain the local parser and verifier proofs; fuzzing does not replace them.

Strict errcheck's five original findings are repaired. Nonce generation checks
owned secret destruction and returns a zero nonce with a typed joined error if
cleanup fails. The remaining findings were ignored test setup/result facts; the
tests now retain those values and require zero refusal output where applicable.

Benchmarks validate their workload before timing and observe the returned
request, proof, refusal output, or replayed custody afterward. They measure the
same operations; no favorable performance comparison is claimed. Earlier
benchmark outputs remain original evidence even though their result observation
was weaker. Baseline and final CPU/memory profiles and binaries are retained.

## Execution and limits

Evidence is retained under
/work/engineering-evidence/primitive/timeproof-upgrade-20260911. Original failures,
mutation identities, commands, source bytes, committed revision and dirty facts,
Go toolchain, outputs and artifact digests remain append-only. Final validation
uses the committed discovered Timeproof/Compass test scope with cache reuse
disabled, the package's analyzers and a full module build, then serial 30-second
benchmark passes followed by serial 10-second budgets for every discovered fuzz
target. The manifest and external local-integrity report bind the result.
Independent acceptance and external consumer migration are not claimed.

The serial comment now cites the applicable RFC 3161 interoperability width,
not a certificate-serial rule. [RFC 3161 section 2.4.2](https://www.rfc-editor.org/rfc/rfc3161.html#section-2.4.2)
requires clients to accommodate 160-bit serials. That does not establish an
arbitrary total response-size limit.

## Still open: streaming ownership

Timeproof currently owns complete request/response byte slices, clones response
custody, materializes JSON/base64 values and X.509 certificates, and uses fixed
response/certificate admission ceilings. Those are not a demonstrated O(1)
streaming contract for arbitrary response extent. This release neither raises
those limits nor declares them a streaming proof. The next surface is replacing
that aggregate custody/admission path with explicit streamed evidence ownership
and verifying which nominal cryptographic values must remain materialized by the
standard library. Authority-specific trust and signature mechanics must remain
intact during that change. Timeproof stays at the front of the queue.
