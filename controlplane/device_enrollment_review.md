# Device enrollment mechanics: release slice

Primitive owns key-derived installation identity, token-verifier matching,
exact replay commitments, certificate binding, and signature verification.
It does not own license capacity, company membership, API-key issuance policy,
first-use database transactions, device lists, removal, or reset.

`RegistrationRequest` admits a bearer-authorized public-key binding. It does
not itself prove possession of the private key. `VerifyCheckIn` authenticates
the certificate and proves possession through the device signature. A copied
public identity or certificate is insufficient; copied private key material
can clone an installation. This is not hardware identification.

## Proof scope

`TestDeviceEnrollmentCryptographicChainRejectsRebindingAndImpersonation`
executes the real token verifier, certificate issuer, check-in signer and
verifier. It asserts exact identity propagation, unchanged replay commitment,
typed second-installation refusal, and zero proof for a structurally valid
forged signature. This is an in-memory cryptographic integration ratchet,
not a replacement for local layer triads or durable service tests.

`FuzzRegistrationRequestExternalDecoder` now starts with a typed constructed
request and a commitment returned by real admission, not hand-authored valid
JSON or a synthetic pre-filled replay. Every callback checks preserved and
zero receivers on rejection, bounded canonical closure on acceptance, exact
retry identity/commitment/disposition, and typed token versus replay refusal.
Canonical seed, changed nonce, changed key-derived identity, empty/null/missing,
truncation, and maximum byte extent minus one/exact/plus one are retained seeds.
The old generic registration authentication helper is retired.

Related existing local proof surfaces are registration_request decoder triad,
registration authority transaction triad, certificate hostile tests, received
check-in signer-binding triad, signed-document semantic fuzzing, and lease
device-identity derivation vectors/byte/bit tests. Their existence does not
establish a completed package sweep or automatically satisfy numeric floors.
This slice does not add a producer/classifier split or new production struct.

## Auditable red states and attempts

Local retained attempt directories are under
`/private/tmp/peachfuzz-state-readonly-evidence/`:

- `primitive-device-enrollment-chain-first`: test fixture failed at structural
  signer binding; not a production defect or semantic mutation result.
- `primitive-device-enrollment-chain-corrected-fixture`: corrected forged
  signature claims the enrolled public key; focused test passed.
- `primitive-device-binding-ratchet-seeds`: integration and rewritten fuzz
  seeds passed; thirteen Go test events, including the fuzz parent.
- `primitive-one-key-rebinding-mutation-red`: disabling
  `resolveRegistrationAuthority`'s prior replay equality rejection makes the
  integration test fail on second-device acceptance. An unused test helper
  was removed during capture; source-before/after hashes retain that fact.
- `primitive-device-signature-mutation-red`: disabling the Ed25519 check in
  `attest.Verify` makes the integration test fail on forged-signature acceptance.

Both production mutations were restored. Each attempt retains base revision,
dirty source snapshot, argv, scope, toolchain, cache posture, exit, output and
artifact hashes. Failures are not overwritten by later passes.

## Remaining acceptance and product surfaces

Local evidence is not independent acceptance. The local diagnostic recorder
does not infer planned/not-run identities from absent Go events and cannot
issue a full-doctrine acceptance receipt. Publication is not certification.

The product must separately prove atomic first use, concurrent contenders,
transaction retries, failed commits releasing no credentials, revocation,
reset and capacity accounting. Those rules must not move into Primitive.
