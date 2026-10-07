# Primitive v2026.1.127 — retained complete-manifest membership

An in-process ManifestAdmission proof cannot survive a server request. The
consumer consequently verified the complete manifest again for every retrieved
object, turning a streamed collection into quadratic verification work.

ManifestMembershipIssuance now requires both an authenticated Chit and a
VerifiedManifestEntry produced by a complete fold. It signs one bounded member
under a distinct domain. ManifestMembershipVerification authenticates that
receipt and binds the exact Chit payload, sequence and independently verified
object receipt before restoring the existing verified entry capability. It
returns zero proof on refusal. Collections remain streamed with constant
working memory; this contract adds no total corpus quota or alternate retrieval
implementation.

Development evidence is retained in `work/2026-10-07/manifest-membership/` below
this directory. The five owning packages passed 3,938 Go test events, and chit
plus retrieval passed 2,986 race-enabled test events. Scoped vet and the signed
decoder's three-second semantic fuzz phase passed. The retained-member table
also proves issuance, serialized replay and verification after destroying the
originating admission authority, including a member beyond one catalog page.

The deliberate mutation removing signature verification failed the unsigned
entry-name regression; its failure remains alongside the restored green runs.
An earlier sandboxed attempt failed because the local HTTP listener was denied;
that attempt is retained separately from its permitted rerun. These are scoped
development checks with dirty source bindings, not repository-wide release
gates or independent acceptance. Consumer projection, cloud deployment and
customer installation remain separate operations.
