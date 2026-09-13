# Reusable-key enrollment agreement

`AccessRegistrationRequest` and `AccessRegistrationVerification` distinguish
reusable access keys from the existing one-use registration grant. This is a
separate authentication contract, not a compatibility constructor. The product
reads current key, scope, entitlement and installation replay facts inside its
own transaction. Primitive authenticates the verifier, checks device binding,
and seals the same non-secret registration proof consumed by certificate issuance.
Primitive does not consume the reusable key or decide machine limits or payment.

The request decoder delays secret allocation until strict structural admission
has succeeded. Its bounded raw JSON token is only an internal wire boundary;
public callers share the typed request. A failed decode preserves the receiver.
Verification leaves token custody with the outer transaction owner, which must
destroy it after all retries. Replay belongs to an installation, not to the key.

Local evidence directories under `/private/tmp/peachfuzz-state-readonly-evidence/`:

- `primitive-access-registration-first`: 18 passing test events.
- `primitive-access-registration-verifier-mutation-red`: deliberately ignoring
  a false verifier match fails the foreign-verifier case (two failed events
  including its parent). Mutation discarded.
- `primitive-access-registration-restored-suite`: 6,333 passing and four failed
  events. The authority method inventory needed its new method, and a proposed
  generic replay-buffer wipe violated marshaler ownership. The wipe was removed.
- `primitive-access-registration-contracts-updated`: 6,337 passing events.
- `primitive-access-registration-custody-vet` and
  `primitive-access-registration-custody-green`: build failures from using a
  JSON-v1 raw-message name in the JSON-v2 package. Retained, not test passes.
- `primitive-access-registration-jsontext-vet`: tool exit 0.
- `primitive-access-registration-jsontext-tests`: 1,013 passing test events.
- `primitive-access-registration-semantic-fuzz`: exploratory three-second run;
  retained record owns its outcome.

The semantic fuzz callback checks strict refusal and preserved receiver,
canonical closure and byte ceiling, exact nominal identity, and independent
revealed-byte comparison to the pinned token before accepting authentication.
These tests do not prove product account policy, Firestore transactions, HTTP
routing, purchase settlement or launch acceptance.
Local dirty-tree execution is not independent acceptance; committed-scope checks
must remain distinguishable from these author diagnostics.

## Hostile-boundary audit

The follow-up closes the request's previously unpinned boundary surfaces:

- Ten accepted rows pin independently retained route, build, version, platform,
  nonce and bound-device facts. Changed facts must change the replay commitment.
- Ten rejection rows cover absent/non-object/truncated/trailing documents,
  unknown/duplicate/case-folded members and incorrect token types.
- Twenty boundary rows cover below/at/above/extreme document and token extents,
  token framing, nonce extents/zero/null, installation extents/zero/conflict,
  malformed UTF-8 and excessive depth. Rows are distinct contracts, not added
  spellings of one collapsed producer state. Boundary rows are not also counted
  against the accepted/rejected quotas.
- The decoder projection has a compiler-visible field/type/tag drift guard.
- A real certificate is issued from the verified proof and independently
  verified. Its exact fields are asserted, an account-only forgery is rejected
  with a zero proof, and absent registration proof cannot issue a certificate.
- Fuzzing includes a genuinely valid foreign token and below/at/above ceiling
  documents, with authentication and canonical closure checked in the callback.
- The benchmark fixes its six-field workload before timing, reports bytes and
  allocations and checks an observed proof after timing. No performance
  improvement is claimed from the shared developer machine.

`primitive-access-registration-hostile-boundaries-first` retained 42 passing
events. `primitive-access-doctrine-pinned` ran the gate's pinned Witness version
`v0.0.0-20260803211814-57582de85018` over the two touched package directories and
exited 0; it did not substitute the developer's dirty installed binary.
`primitive-access-audit-vet` exited 0. The deliberately widened decoder limit in
`primitive-access-document-ceiling-mutation-red` failed the above-ceiling case
(two failed events including its parent); the mutation was discarded.

This boundary validates/authenticates one request against a verifier and an
optional exact replay; it is not a producer emitting evidence batches into a
policy classifier. The 50-case producer/classifier matrix is not substituted for
these parser/authentication contracts. Fresh/exact replay and typed refusal are
proved by their own tests; no batch classification lattice is invented.

The canonical script's benchmark budget was corrected from three to 30 seconds,
and execution order corrected to tools/tests, benchmarks, then fuzz targets.
This does not turn the script's local success or GitHub admission notification
into an independent acceptance receipt. The script's broader execution accounting
and the independent authority's receipt must still be audited before certifying
the whole repository. Product integration remains a separate proof surface.
