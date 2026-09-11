# Timeproof status and DER custody slice — v2026.1.68

This slice fixes two status-text admission defects and removes three full DER
copies. Timeproof remains open for aggregate response and certificate ownership.
Baseline is v2026.1.67, e16e1c7255877c673df0fef929b8a3e2fed03f1d.

The old status-text loop rejected a ninth text entry while accepting invalid
UTF-8 bytes inside a UTF8String. RFC 3161 uses PKIFreeText, whose sequence has
one or more UTF8String elements, not an eight-element ceiling. The current ASN.1
module spells out that unbounded cardinality in
[RFC 5912](https://www.ietf.org/rfc/rfc5912.html).
The parser now advances through caller-owned DER spans and validates each
string's UTF-8 in place. No string list, growing counter, or artificial element
ceiling is needed. An empty sequence is still malformed; a sequence containing
one empty UTF8String is a distinct admitted representation.

The real Verify path has local positive/negative/neutral tests for status text.
Ninth and thousandth valid entries previously failed; invalid byte, truncated
rune, overlong encoding and surrogate encoding previously became typed
refusals. Those six failures are retained. A standard-library ASN.1 struct decode
inside both provider fuzz callbacks independently checks refused source syntax,
status and failure-code inclusion. Its malformed UTF-8 seed fails the previous
production code. The secondary fuzz model is bounded by Verify's existing
response contract; it is test-only and is not the production streaming design.

parseTimestampResponse, explicitOctets and consumeFinalSignature now return
read-only spans into the caller-owned input rather than copying the entire CMS
token, TSTInfo, and signature. Three independent address/byte-span assertions
failed before the change and pass afterward. These private handoffs do not
extend beyond synchronous verification. Real Verify still creates independently
owned evidence: mutating the original response and a returned byte accessor
after verification cannot change the proof or its re-verifiable JSON. Removing
that durable copy is a recorded failing semantic mutation, discarded afterward.

The old test that demanded rejection at nine status-text entries is retired.
The production cap constant is removed. The transport-free production import
inventory now includes unicode/utf8. No new production struct, provider policy,
product state, compatibility wrapper, or graph is introduced.

## Execution evidence

/work/engineering-evidence/primitive/timeproof-status-stream-20260911 retains
all attempts, source bytes and dirty facts, commands, exact committed revisions,
toolchain/machine facts, exit statuses, stdout/stderr and artifact hashes.
An initial attempt named status-text-fuzz-red actually ran the unchanged oracle
after a setup assertion failed; setup-failure.json explicitly marks it as not a
red state. The corrected status-text-fuzz-red-2 retains the two real failures.
Nothing is overwritten or reclassified as a passing proof.

Final validation discovers all Timeproof/Compass tests and all Timeproof fuzz
targets and benchmarks. Tools and uncached race tests precede serial 30-second
benchmarks with retained CPU/memory profiles and binary, then serial 10-second
fuzz budgets. The external integrity report binds those facts to the manifest.
The new status validator benchmark measures 1 KiB and 1 MiB inputs with fixed
working storage. Before/after timing is not an improvement claim: the old code
did not inspect UTF-8 content, while correct work must scale with input bytes.
No machine power posture or independent acceptance is claimed.

## Remaining boundary

This is O(1) auxiliary memory for the status-text walk and borrowed DER handoffs,
not a complete io.Reader-based timestamp verifier. Public response/evidence
custody still materializes []byte, certificates still enter x509's nominal
objects/pools, and response/collection ceilings remain in the older verifier.
Those surfaces remain open. The caller must keep borrowed input immutable
during Verify; only the returned verified evidence owns its durable copy.
