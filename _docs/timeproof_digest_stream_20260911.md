# Timeproof digest declaration slice — v2026.1.69

Baseline is v2026.1.68, 34083fba6a2bb6e49b319d272a616d1b67470d32.
Timeproof remains open; this slice replaces one aggregate handoff.

The old parser accumulated digest algorithm declarations in a slice and rejected
a fifth declaration. RFC 5652 section 5.1 permits any collection cardinality:
https://www.rfc-editor.org/rfc/rfc5652.html#section-5.1
An authentic timestamp with four foreign declarations plus its signer digest
failed before this change, as did the 1024-foreign-declaration case. The new
parser validates each declaration in place and retains one borrowed DER span.
The signer lookup walks that span with one boolean: missing and duplicate
matching declarations still reject. Unknown identifiers do not supply a signer
digest. Unknown OID arcs are checked as canonical base-128 bytes without integer
conversion, growing lists or artificial arc-width limits. Algorithm parameters
remain opaque ASN.1 spans; a second parameter is rejected rather than ignored.

The parsedSignedData inventory role remains internal flow; its digest field is
now an ASN.1 span rather than a list of algorithm objects. No production struct,
compatibility wrapper, provider policy or product state machine was added.

## Proof surfaces

TestDigestDeclarationLayerTriad rebuilds only the unsigned declaration set of a
real authentic CMS response, then calls Verify and compares the signed facts.
Absence and duplicates return the typed invalid identity and an entirely zero
proof. TestDigestSetEncodingLayerTriad exercises the local parser's grammar,
neutral empty set and exact borrowed-span ownership; it is direct unit proof.
These are earned boundary cases for this slice, not a claimed complete package
10/10/20 sweep or a producer/classifier 50-case matrix.

FuzzDigestOIDRepresentation compares the real declaration parser against
crypto/x509.OID, independently checks identity matching and canonical bytes,
and requires zero spans with typed errors on refusal. Its secondary oracle is
bounded by the current public response contract; larger input executes Verify
and proves its existing typed size refusal. This documents the remaining
public ceiling, not a streaming completion claim. The existing FreeTSA response
fuzzer also receives verified custody seeds with enlarged declaration sets and
a duplicate-digest negative seed, retaining its authentic signed-agreement
oracle. DigiCert continues to exercise its unchanged real response boundary.

Two discarded one-fact mutations prove the minimal-OID and duplicate-digest
assertions fail. The initial digest-set-red attempt was a test compilation
failure, retained as such; digest-set-red-2 contains the actual production red
state. Lint's first two shadowing findings are also retained and corrected.

## Execution and remaining work

/work/engineering-evidence/primitive/timeproof-digest-stream-20260911 retains
every command, revision, dirty fact, attempt, output and artifact digest. The
committed gate runs tools and uncached race tests, then serial 30-second
benchmarks with profiles and binary, then all discovered fuzz targets serially.
The declaration benchmark compares fixed workloads of 4 and 4096 foreign
declarations; no performance improvement or machine power posture is claimed.
Manifest verification is local integrity, not independent acceptance.

Response/evidence custody still owns whole byte slices. Certificates and signed
attributes still have aggregate paths and older ceilings. Those are the next
Timeproof surfaces; no complete O(1) public timestamp verifier is claimed.
