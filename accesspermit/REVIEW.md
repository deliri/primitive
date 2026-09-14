# Scoped access agreement

Primitive owns authenticated binding, canonical encoding, byte bounds and
half-open time-window mechanics. The caller supplies the decision and all three
window instants; Primitive does not know what an allowed operation means.
Registration and check-in response carriers combine existing Primitive documents
with this independent permit. A current lease never extends an expired permit.

Production structs are classified by TestProductionStructInventory. External
ingresses and semantic fuzz targets:

- Document.Decode/UnmarshalJSON: FuzzPermitAuthenticatedSemanticClosure.
- Decision.UnmarshalJSON: FuzzPermitDecisionSemanticDomain.
- Domain.ParseCanonicalText: FuzzPermitDomainCanonicalText.
- RegistrationResponse.UnmarshalJSON: FuzzRegistrationResponseSemanticBinding.
- CheckInResponse.UnmarshalJSON: FuzzCheckInResponseSemanticBinding.
- Signed typed input to Verify: FuzzPermitSignedFactMutations.

The issuer, schema, decoder, verifier, response binding and canonical writer have
local LayerTriad tests. Real ed25519 signing and Primitive verification drive the
fixtures. Refusals return zero proof, malformed decode preserves the receiver,
and authentication is equivalent to membership in the genuine signed seed set.
This package performs no durable writes, provider effects, ledger classification
or product-state transitions; those downstream owners need their own evidence.

Local evidence is retained under
/private/tmp/peachfuzz-state-readonly-evidence. These are development diagnostics,
not independent acceptance or a claim that the base commit contains dirty bytes.
Each execution.json retains its complete command, source snapshots, base commit,
dirty state, environment, process status, output sizes and hashes. Test event
counts include parent tests; unavailable and not-run identities remain unknown.

- primitive-access-permit-expiry-mutation-red: changed the half-open expiry
  comparison from >= to >; the exact-expiry case failed. Mutation discarded.
- primitive-permit-core-inventory: 13,299 passed events across core, accesspermit,
  controlwire, controlplane and submission; zero failed or skipped events.
- primitive-permit-auth-equivalence: 114 passed events under the race detector.
- primitive-permit-final-vet/staticcheck/doctrine: exit zero.
- primitive-permit-final-benchmark: fixed real verification, allocations reported,
  3-second configured measurement; no performance improvement claim.
- primitive-permit-final-Fuzz*: six targets, once each, 2-second exploration plus
  1-second minimization allowance, one worker and GOMAXPROCS=1; exit zero.

Earlier failures remain retained. The broad core inventory exposed missing
validation witnesses and stale mechanical-call counts. Counts were reconciled
against d5de09b (three content-sort SameFile comparisons) and 42e6179 (read-only
process-group signal-zero observation), not increased to conceal new effects.

Release acceptance still requires independent execution of the exact committed
revision. A local pass, tag or submission acknowledgment is not that receipt.
