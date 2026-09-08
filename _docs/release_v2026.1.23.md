# Primitive v2026.1.23

This user-reviewed checkpoint publishes the Keygen upgrade on Go 1.27.1.

Private-key adoption now rejects inconsistent Ed25519 seed/public halves instead
of silently repairing supplied input. Refusals preserve typed error identities,
zero results and caller bytes. The internal Go-result adoption boundary also
checks the separately supplied public result against the private-key suffix.
Owners of invalid persisted keys must correct them explicitly.

Go continues to own entropy and Ed25519; Core owns secret custody. CPU profiles
identified duplicate derivation in private-key projection. That path now derives
once and validates the returned result. Comparable Core public values remove an
unnecessary allocation without adding a cache or cryptographic implementation.

Keygen passed 1,022 leaf tests under race detection and two serial fuzz campaigns
totaling 714,644 executions. Ten before/after benchmark pairs retain requested
30-second workloads, CPU/memory profiles and matching binaries. Private projection
fell from three allocations to two; validation fell from two to one. Timing
samples are reported without claiming statistical significance. See the
[review](../keygen/upgrade_review.md) and [evidence](../keygen/upgrade_evidence.json).

Recorded scoped analyzers passed. Darwin/arm64 tests ran natively; Linux/amd64
and Windows/amd64 checks compiled test binaries only. Full-module gates were not
repeated. Fresh Compass and Version coordinate tests are bound in the
[release evidence](release_v2026.1.23_evidence.json). Large raw profiles, binaries
and command logs remain ignored locally; reports are included in Git.

The user explicitly approved this slice and authorized bump, commit and push.
Release is the next package. Consumer dependency pins are unchanged by this
publication, and remaining packages are not represented as reviewed.
