# Reusable bearer contract slice

`AccessToken` and `AccessTokenVerifier` are mechanical contracts. They contain no
company, user, purchase, entitlement, machine limit, expiry, revocation, workflow,
or acceptance policy. `Matches` proves token identity only. Product code must
authenticate account ownership and check its current authorization records.

The product uses `keygen.GenerateSecret` for issuance and explicitly constructs
the fixed-width token. This package does not generate product accounts or issue
registration grants. The existing one-use `RegistrationToken` remains distinct.
Both customer apps and the API must import the same released Primitive version
before these new contracts can be wired into a shipped product.

## Proof surfaces

- Schema/identity: `TestAccessTokenIdentityLayerTriad` checks the independent
  SHA-256 derivation, repeated-use identity, one-bit mismatch, absent inputs and
  rejection by the one-use token parser. No token match claims purchase or access.
- Text ingress: `TestAccessTokenParserExhaustsEveryPositionAndByte` checks every
  byte at every position of the fixed-width grammar and adjacent length bounds.
  This is a factored grammar check, not an enumeration of all 256-bit secrets and
  not a claim to meet policy-table quotas by counting alphabet substitutions.
- Secret ownership: `TestAccessTokenCopiesRedactAndDestroyTogether` checks caller
  buffer isolation, formatting redaction and invalidation of copied handles.
- Text/JSON: `FuzzAccessTokenTextAndJSONSemanticClosure` uses typed emitted seeds,
  an independent standard-library decoder/re-encoder oracle, exact accepted
  bytes, canonical closure, and zero/preserved receivers on typed refusal.
- Verifier JSON: `FuzzAccessTokenVerifierJSONSemanticClosure` independently checks
  canonical digest admission, zero/preserved refusal and exact round trips.
- Existing AST inventories name both carriers and both dedicated fuzz targets.

The deliberate semantic mutation omitting the verifier domain copy failed
`TestAccessTokenIdentityLayerTriad`. The domain was restored before the green
package run. Local retained attempts are under
`/private/tmp/peachfuzz-state-readonly-evidence/primitive-access-token-*` and include
source bytes, command, toolchain, cache posture, stdout/stderr hashes and exit.
These are local diagnostics, not independent acceptance receipts. Test-event
counts include parents; the harness does not infer unavailable or not-run tests
from missing events. No benchmark improvement, production launch, customer
purchase, database token issuance or app enrollment is claimed by this slice.

Remaining integration proof is the real account authorization -> verified
purchase -> verifier-only persistence -> revocation-aware enrollment path,
including commit refusal, replay, distinct machines and exact scoped receipts.
That is product policy and durable integration work, not a new Primitive state
machine. No deployment or running binary replacement is authorized by this note.
