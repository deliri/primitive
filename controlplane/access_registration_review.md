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
routing, purchase settlement or launch acceptance. Numeric hostile-table floors
and the wider package doctrine gate are not claimed closed by this narrow slice.
Local dirty-tree execution is not independent acceptance; committed-scope checks
must remain distinguishable from these author diagnostics.
