# Primitive v2026.1.31

Controlplane now preserves exact document error ownership for canonical writes,
refuses received check-ins whose signer differs from the certified device key,
and preserves check-in identity on signing-domain consistency failures. Short
writes cannot report completion, nil destinations refuse, and failed issuance
returns a zero request. Registration authority cleanup consumes tokens on every
exit. Conflict responses reject watermarks their producer would accept or replay.

Generic authenticated responses retain private bounded wire bytes and return
independently decoded typed bodies. Caller mutation cannot rewrite retained
proofs. Response commitments enforce the document extent and exact empty-body
SHA-256. Public method signatures and wire shapes remain unchanged; no new
runtime, dependency, or product policy was added.

Hostile tables exercise exact writer identities, receiver preservation,
signer/certificate closure, byte ceilings, and producer transitions. The final
Linux race run records 997 passing events, no failures or skips. Scoped vet,
Staticcheck, Errcheck, Witness, production complexity, and Darwin/Windows
compilation pass. The original 21-target fuzz campaign remains historical; all
seeds execute in the final race run and the changed check-in ingress receives a
fresh 30-second campaign with 128,787 reported executions. Full-module gates,
native Darwin/Windows execution, and consumer upgrades were not run.

Ten benchmark workloads retain original, initial-candidate, and current results,
CPU/memory profiles, and matching binaries outside Git. Independent response body
extraction costs 444,651 ns/op, 69,132 B/op, and 1,118 allocations in the current
sample, versus the original aliased result's 8,578 ns/op, 2,289 B/op, and 27
allocations. Response verification drops from 174 to 122 allocations. These are
single shared-server observations, not a latency trend. The enum parser hits
Go's iteration cap before 30 seconds and remains informative only.

The user reviewed the slice and explicitly approved bump, commit, and push.
Concurrent v2026.1.30 Manual changes were fast-forwarded before this release;
Controlplane's reviewed files and production dependency sources remain unchanged.
Release-coordinate tests for Compass and Version pass. Consumer pins remain
unchanged. Shutdown is the next package in the saved queue.

See [the reviewed report](controlplane_upgrade_20260909.md),
[the current source evidence](controlplane_upgrade_20260909_review_followup.json),
and [release evidence](release_v2026.1.31_evidence.json).
