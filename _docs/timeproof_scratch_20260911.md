# Timeproof scan-owned decoder storage — v2026.1.70

Baseline is v2026.1.69, 1768ada1a4c707e8109525e9cdd828c9a7a29157.
The previous digest scan removed the aggregate list, but each call to
asn1.Unmarshal still allocated a fresh raw-value destination.

Status-text and digest-declaration walks now reuse a scan-owned ASN.1 destination.
The standard-library decoder remains responsible for DER admission. No custom
DER runtime, new wire type or new size limit is introduced. Single-value callers
still own their own temporary; collection walkers explicitly pass their reusable
storage. Returned raw values remain borrowed spans into the input, independent
of later destination reuse. Refusal clears both the result and the destination.

TestDERScanAllocationsDoNotScaleWithElementCount uses typed process isolation
because testing.AllocsPerRun controls runtime allocation measurement. It compares
4 and 4096 elements under the same compiled toolchain, without an arbitrary
allocation quota. Before the change, digest scans grew from 12 to 8196 allocations
and status scans from 4 to 4096. Both growth checks failed red and now pass.
Each measured invocation checks nonempty workload completion and its error.

TestRawDecodeScratchLayerTriad proves earlier borrowed values survive reuse,
a malformed value clears a populated destination with typed rejection, and
empty octets replace earlier bytes without losing their own framing. Removing
the scratch-reset invariant makes the refusal case fail; that semantic mutation
is retained in evidence and discarded from production.

The public provider and JSON fuzz oracles remain applicable because all raw DER
reads still use the same standard decoder. The existing OID differential oracle,
full authentic signature/binding assertions and zero-proof assertions exercise
the changed handoff. No external door or production struct was added.

## Evidence and remaining work

/work/engineering-evidence/primitive/timeproof-scratch-20260911 retains all
attempts, exact source snapshots/revisions, commands, cache posture, outcomes,
stdout/stderr and artifact hashes. The committed gate runs package tools and
uncached race tests, all serial 30-second benchmarks with profiles and binary,
then all five fuzz targets serially. Local manifest integrity is not independent
acceptance. Timing improvements and machine power posture are not claimed.

This closes per-element destination allocation in two loops. Whole-response
custody, certificate collections, signed-attribute collections and other older
aggregate paths remain open. Timeproof is not a completed streaming verifier.
