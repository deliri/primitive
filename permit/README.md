# Authenticated action permits

`permit` owns the shared signed agreement for a bounded set of opaque action
identifiers. Callers select actions and validity terms; Primitive authenticates
their exact identity, subject, build, nonce, generation, and invocation window.
It does not assign actions to plans or dispatch product commands.

`RegistrationResponse` and `CheckInResponse` bind the existing control-plane
documents to the same issued permit. Both independently deployed consumers use
these structs. Structural decoding still requires independent signature and
binding verification before a caller receives `Verified` authority.

The package uses owned fixed-size action sets. Empty grants authorize nothing;
union is idempotent for an existing member and refuses overflow without mutation.
Remote operation identities derive from the shared `controlwire.RouteFamily`.

## Verification scope

Local diagnostic evidence is retained under
`/private/tmp/bug-primitive-permit-evidence-20260913` on the author's workspace.
It includes source snapshots, complete commands, output digests, and every
attempt. It is not an independent acceptance receipt.

The membership ratchet was demonstrated by deliberately bypassing the absent
action rejection in `Verified.Allows`; `TestPermitVerificationLayerTriad` failed.
That mutation was discarded. The route test also exposed an actual construction
bug: URL paths cannot serve as action identifiers; the shared canonical route
token now supplies that identity.

Eight semantic fuzz targets cover the public representation boundaries. Their
30-second local phases all completed successfully. Signed document oracles use
the real attestation verifier; refusal oracles require typed errors and zero or
unchanged results. The struct and external-ingress inventories are executable
ratchets in `inventory_test.go`.

The complete local core/permit race run recorded 2,110 test events: 2,108 passed
and two failed. The new package's missing marshaler witnesses were then added.
The remaining two core failures (effect ownership and existing marshaler
witnesses in other packages) reproduce with identical diagnostics on clean
v2026.1.85 at `c87eada049b9035a08cc1980c2f72ff9cefdf3d6`; both comparison
attempts are retained. This is an explicit baseline, not a green core claim.

This is a new wire domain. It does not decode legacy product-owned permission
documents. Consumers must explicitly migrate persisted agreements and preserve
existing paid obligations through their own policy. No compatibility authority
or inferred replacement grant is supplied here.
